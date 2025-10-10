/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2025.
*/

package cmd

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "math/rand"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "syscall"
    "time"

    "github.com/Azure/azure-storage-fuse/v2/common"
    "github.com/Azure/azure-storage-fuse/v2/common/log"
    "github.com/Azure/azure-storage-fuse/v2/component/attr_cache"
    "github.com/Azure/azure-storage-fuse/v2/component/block_cache"
    "github.com/Azure/azure-storage-fuse/v2/component/file_cache"
    "github.com/Azure/azure-storage-fuse/v2/internal"
    "github.com/spf13/cobra"
)

type invalidateRequest struct {
    Version   int      `json:"version"`
    Op        string   `json:"op"`
    MountRoot string   `json:"mount_root"`
    Scope     string   `json:"scope"`
    Recursive bool     `json:"recursive"`
    Paths     []string `json:"paths"`
}

var (
    invScope     string
    invRecursive bool
)

// mountInvalidateCmd invalidates local caches for given paths on a specific mount
var mountInvalidateCmd = &cobra.Command{
    Use:   "invalidate <mountpoint> <path...>",
    Short: "Invalidate caches for paths under a mountpoint",
    Long:  "Invalidate attribute/file/block caches for the specified paths under the given mountpoint.",
    Args: func(cmd *cobra.Command, args []string) error {
        if len(args) < 2 {
            return fmt.Errorf("requires a mountpoint and at least one path")
        }
        if _, err := validateScope(invScope); err != nil {
            return err
        }
        return nil
    },
    RunE: func(cmd *cobra.Command, args []string) error {
        mountPoint := args[0]
        paths := args[1:]

        // Normalize mountpoint to absolute path for matching
        absMount, err := filepath.Abs(mountPoint)
        if err != nil {
            return fmt.Errorf("failed to resolve mountpoint: %w", err)
        }

        // Ensure mount exists in system mounts
        lst, err := common.ListMountPoints()
        if err != nil {
            return fmt.Errorf("failed to list mount points: %w")
        }
        found := false
        for _, m := range lst {
            if m == absMount {
                found = true
                break
            }
        }
        if !found {
            // best-effort: still continue; user might be running on non-Linux or mount not visible in /etc/mtab
        }

        pid, err := findPidByMountpoint(absMount)
        if err != nil {
            return err
        }

        req := invalidateRequest{
            Version:   1,
            Op:        "invalidate",
            MountRoot: absMount,
            Scope:     invScope,
            Recursive: invRecursive,
            Paths:     make([]string, 0, len(paths)),
        }

        for _, p := range paths {
            p = strings.TrimSpace(p)
            if p == "" {
                continue
            }
            // standardize as mount-relative path (remove any leading slash)
            req.Paths = append(req.Paths, strings.TrimLeft(p, "/"))
        }
        if len(req.Paths) == 0 {
            return fmt.Errorf("no valid paths provided")
        }

        if err := publishInvalidateRequest(pid, &req); err != nil {
            return err
        }

        // Send signal to nudge the target
        if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
            // If signal fails (platform/permission), do not fail hard; request file remains for scanners
            fmt.Fprintf(cmd.ErrOrStderr(), "warn: failed to send SIGUSR1 to %d: %v\n", pid, err)
        }

        fmt.Fprintf(cmd.OutOrStdout(), "invalidate request submitted to pid %d for %d path(s)\n", pid, len(req.Paths))
        return nil
    },
}

func init() {
    mountCmd.AddCommand(mountInvalidateCmd)
    mountInvalidateCmd.Flags().StringVar(&invScope, "scope", "all", "Invalidate scope: attr|file|block|all")
    mountInvalidateCmd.Flags().BoolVar(&invRecursive, "recursive", false, "Recursively invalidate directories")
}

func validateScope(s string) (string, error) {
    switch strings.ToLower(s) {
    case "attr", "file", "block", "all":
        return strings.ToLower(s), nil
    default:
        return "", fmt.Errorf("invalid scope: %s", s)
    }
}

// findPidByMountpoint finds the blobfuse2 process whose command line contains the mountpoint
func findPidByMountpoint(mountpoint string) (int, error) {
    var out bytes.Buffer
    cmd := exec.Command("pidof", "blobfuse2")
    cmd.Stdout = &out
    if err := cmd.Run(); err != nil {
        if err.Error() == "exit status 1" {
            return 0, errors.New("no blobfuse2 process found")
        }
        return 0, fmt.Errorf("failed to get pid of blobfuse2: %w", err)
    }

    pidString := strings.ReplaceAll(out.String(), "\n", " ")
    pids := strings.Split(pidString, " ")
    myPid := os.Getpid()

    matches := make([]int, 0, 2)
    for _, p := range pids {
        p = strings.TrimSpace(p)
        if p == "" {
            continue
        }
        ipid, _ := strconv.Atoi(p)
        if ipid == myPid {
            continue
        }
        out.Reset()
        cmd = exec.Command("ps", "-o", "args=", "-p", p)
        cmd.Stdout = &out
        if err := cmd.Run(); err != nil {
            continue
        }
        if strings.Contains(out.String(), mountpoint) {
            matches = append(matches, ipid)
        }
    }

    if len(matches) == 0 {
        return 0, fmt.Errorf("no matching mount process found for %s", mountpoint)
    }
    if len(matches) > 1 {
        return 0, fmt.Errorf("multiple mount processes match %s: %v", mountpoint, matches)
    }
    return matches[0], nil
}

func publishInvalidateRequest(pid int, req *invalidateRequest) error {
    // ctrl dir: ~/.blobfuse2/ctrl/<pid>
    ctrlBase := filepath.Join(common.ExpandPath(common.DefaultWorkDir), "ctrl", strconv.Itoa(pid))
    if err := os.MkdirAll(ctrlBase, 0o700); err != nil {
        return fmt.Errorf("failed to create control dir: %w", err)
    }

    // unique filename
    ts := time.Now().UnixNano()
    rnd := rand.Uint32()
    fname := fmt.Sprintf("invalidate-%d-%d.json", ts, rnd)
    tmp := filepath.Join(ctrlBase, ".tmp-"+fname)
    fin := filepath.Join(ctrlBase, fname)

    f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
    if err != nil {
        return fmt.Errorf("failed to create request file: %w", err)
    }
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    if err := enc.Encode(req); err != nil {
        _ = f.Close()
        return fmt.Errorf("failed to write request: %w", err)
    }
    if err := f.Sync(); err != nil {
        _ = f.Close()
        return fmt.Errorf("failed to fsync request: %w", err)
    }
    if err := f.Close(); err != nil {
        return fmt.Errorf("failed to close request: %w", err)
    }
    if err := os.Rename(tmp, fin); err != nil {
        return fmt.Errorf("failed to publish request: %w", err)
    }
    return nil
}

// processOutstandingInvalidateRequests scans control directory for pending invalidate requests and processes them.
// For the initial iteration, it validates and logs the requests, and then removes them.
func processOutstandingInvalidateRequests(pipeline *internal.Pipeline) error {
    pid := os.Getpid()
    ctrlBase := filepath.Join(common.ExpandPath(common.DefaultWorkDir), "ctrl", strconv.Itoa(pid))
    dir, err := os.ReadDir(ctrlBase)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }

    for _, de := range dir {
        name := de.Name()
        if !de.Type().IsRegular() || !strings.HasPrefix(name, "invalidate-") || !strings.HasSuffix(name, ".json") {
            continue
        }
        fp := filepath.Join(ctrlBase, name)
        data, err := os.ReadFile(fp)
        if err != nil {
            log.Warn("invalidate: failed to read %s: %v", fp, err)
            continue
        }
        var req invalidateRequest
        if err := json.Unmarshal(data, &req); err != nil {
            log.Warn("invalidate: failed to parse %s: %v", fp, err)
            _ = os.Remove(fp)
            continue
        }

        // Basic validation
        if strings.ToLower(req.Op) != "invalidate" || len(req.Paths) == 0 {
            log.Warn("invalidate: invalid request in %s", fp)
            _ = os.Remove(fp)
            continue
        }

        // Mount guard: ensure request targets this mount
        // options.MountPath is absolute and set during mount
        if req.MountRoot != "" && filepath.Clean(req.MountRoot) != filepath.Clean(options.MountPath) {
            log.Info("invalidate: skipping request for different mount %s (this: %s)", req.MountRoot, options.MountPath)
            _ = os.Remove(fp)
            continue
        }

        scope, err := validateScope(req.Scope)
        if err != nil {
            log.Warn("invalidate: bad scope in %s: %v", fp, err)
            _ = os.Remove(fp)
            continue
        }

        // For now, only log the request. Integration with components will follow in the next iteration.
        log.Info("invalidate: scope=%s recursive=%t paths=%v", scope, req.Recursive, req.Paths)

        // Discover cache components from the pipeline
        var ac *attr_cache.AttrCache
        var fc *file_cache.FileCache
        var bc *block_cache.BlockCache
        if pipeline != nil && pipeline.Header != nil {
            for c := pipeline.Header; c != nil; c = c.NextComponent() {
                switch t := c.(type) {
                case *attr_cache.AttrCache:
                    ac = t
                case *file_cache.FileCache:
                    fc = t
                case *block_cache.BlockCache:
                    bc = t
                }
            }
        }

        // Apply invalidation
        for _, p := range req.Paths {
            mp := strings.TrimLeft(p, "/")
            if req.Recursive {
                if ac != nil && (scope == "attr" || scope == "all") {
                    ac.InvalidateDirExt(mp)
                }
                if fc != nil && (scope == "file" || scope == "all") {
                    fc.InvalidateDirExt(mp)
                }
                if bc != nil && (scope == "block" || scope == "all") {
                    bc.InvalidateDirExt(mp)
                }
            } else {
                if ac != nil && (scope == "attr" || scope == "all") {
                    ac.InvalidatePathExt(mp)
                }
                if fc != nil && (scope == "file" || scope == "all") {
                    fc.InvalidateFile(mp)
                }
                if bc != nil && (scope == "block" || scope == "all") {
                    bc.InvalidateFile(mp)
                }
            }
        }

        // Best-effort: remove after processing
        if err := os.Remove(fp); err != nil {
            log.Warn("invalidate: failed to remove %s: %v", fp, err)
        }
    }
    return nil
}

// startInvalidateScanner starts a low-frequency scanner to pick pending requests
// in case signals are missed or unavailable. It exits when the process ends.
func startInvalidateScanner(pipeline *internal.Pipeline) {
    interval := 2 * time.Second
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            _ = processOutstandingInvalidateRequests(pipeline)
        }
    }()
}
