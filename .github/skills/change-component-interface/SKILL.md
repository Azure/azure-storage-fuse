---
name: change-component-interface
description: 'Add, remove or change a method or options struct of the pipeline Component interface in internal/component.go. Use when a new file operation must flow through the pipeline, when an options struct (CreateFileOptions, ReadInBufferOptions, ...) needs a new field, or when a signature changes. Covers BaseComponent, the hand-maintained gomock MockComponent, the exported package for custom components, libfuse callers, and the changelog.'
---

# Change the Component interface

Every component, the base implementation, the test mock and external plugin components depend on `internal.Component`. Compile-time assertions (`var _ internal.Component = &X{}`) make missing methods fail the build, so `make check` finds most places you missed.

## Prefer a new options field

Operations take an options struct from `internal/component_options.go`, for example `DeleteFileOptions`. Adding a field is backward compatible: components that don't read it keep working, and the zero value must mean "old behavior". Change a method signature or add a method only when a field can't express the change.

## Steps

1. Change `internal/component.go`, and the struct in `internal/component_options.go` if needed. Document any contract in a comment on the method, as `GetAttr` and `CreateFile` do.
2. Add or update the forwarding implementation in `internal/base_component.go`, following the existing pattern:

   ```go
   func (base *BaseComponent) DeleteFile(options DeleteFileOptions) error {
   	if base.next != nil {
   		return base.next.DeleteFile(options)
   	}
   	return nil
   }
   ```

3. Update `internal/mock_component.go` by hand. It is gomock output that has been edited, for example `RenameFile` sets `DstAttr` times, so don't overwrite it. Generate a reference copy and paste only the new or changed method and its recorder:

   ```bash
   go install github.com/golang/mock/mockgen@v1.6.0
   $(go env GOPATH)/bin/mockgen -self_package=github.com/Azure/azure-storage-fuse/v2/internal -package=internal \
     -destination=/tmp/mock_component.go github.com/Azure/azure-storage-fuse/v2/internal Component
   ```

4. Add a type alias for any new options type in `exported/exported.go`. External components loaded from `.so` plugins by `component/custom` implement the interface through that package.
5. Implement the method in each component that must act on it; components that only pass the call on get it from `BaseComponent`. Find existing implementations with `grep -rn 'func (.*) <Method>(' component/`. Remember `azstorage` has two backends, `block_blob.go` and `datalake.go`.
6. If FUSE triggers the operation, call it from both `component/libfuse/libfuse_handler.go` and `libfuse2_handler.go`, and map its errors as described in the libfuse instructions.
7. Add unit tests in each component that implements the method; use `MockComponent` expectations to check the call reaches the next component.
8. Add a `CHANGELOG.md` entry. If custom components must change, say what their authors have to do, as the 2.5.6 entry about `ReadLinkOptions.Size` does, and consider listing it under **Breaking Changes**.

## Verify

```bash
make check                                    # both FUSE variants, all tests compile
make test PKG=./internal/...
make test PKG=./component/<changed component>
make verify
```
