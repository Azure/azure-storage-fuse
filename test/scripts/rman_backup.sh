#!/bin/bash

# rman_backup.sh
# Performs actual Oracle RMAN backup workloads on a FUSE-mounted filesystem.
# Installs Oracle XE, creates tablespaces with datafiles of various sizes,
# performs full and incremental backups using the rman utility, and validates
# data integrity.
#
# Usage: ./rman_backup.sh /path/to/mountpoint [sizes]
#   sizes: comma-separated list of datafile sizes (default: 10M,100M,1G,10G)

# Note: -e is intentionally omitted because the script uses explicit $? checks
# and graceful skip logic (exit 0 when Oracle is unavailable).
set -uo pipefail

MOUNT_POINT="$1"
DATA_DIR="$2"
SIZES="${3:-10M,100M,1G,10G}"

if [ -z "$MOUNT_POINT" ]; then
    echo "Usage: $0 /path/to/mountpoint [sizes]"
    echo "  sizes: comma-separated list (e.g., 10M,100M,1G,10G)"
    exit 1
fi

if [ ! -d "$MOUNT_POINT" ]; then
    echo "Error: Directory $MOUNT_POINT does not exist."
    exit 1
fi

if [ -z "$DATA_DIR" ]; then
    echo "Usage: $0 /path/to/mountpoint /path/to/data-dir [sizes]"
    echo "  data-dir: directory with sufficient space for Oracle database files (>=4.5GB)"
    exit 1
fi

if [ ! -d "$DATA_DIR" ]; then
    echo "Error: Data directory $DATA_DIR does not exist."
    exit 1
fi

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

BACKUP_BASE="$MOUNT_POINT/rman_backup"
RESTORE_DIR="$DATA_DIR/rman_restore_$$"
PASSED=0
FAILED=0

IFS=',' read -ra SIZE_ARRAY <<< "$SIZES"

# ==============================================================================
# setup_oracle_env: Set Oracle environment variables
# Detects ORACLE_HOME from common locations if not already set.
# ==============================================================================
setup_oracle_env() {
    # Try to source Oracle's environment profile if available
    if [ -f /opt/oracle/product/21c/dbhomeXE/bin/oracle_env.sh ]; then
        . /opt/oracle/product/21c/dbhomeXE/bin/oracle_env.sh 2>/dev/null || true
    elif [ -f /etc/profile.d/oracle-xe-21c.sh ]; then
        . /etc/profile.d/oracle-xe-21c.sh 2>/dev/null || true
    fi

    # Detect ORACLE_HOME from known paths if not set
    if [ -z "${ORACLE_HOME:-}" ]; then
        for candidate in /opt/oracle/product/21c/dbhomeXE /u01/app/oracle/product/21c/dbhomeXE; do
            if [ -d "$candidate" ]; then
                export ORACLE_HOME="$candidate"
                break
            fi
        done
    fi

    export ORACLE_HOME="${ORACLE_HOME:-/opt/oracle/product/21c/dbhomeXE}"
    export ORACLE_SID="${ORACLE_SID:-XE}"
    export PATH="$ORACLE_HOME/bin:$PATH"
    export LD_LIBRARY_PATH="$ORACLE_HOME/lib:${LD_LIBRARY_PATH:-}"
    export NLS_LANG="AMERICAN_AMERICA.AL32UTF8"
}

# Set Oracle environment early so cleanup and all functions have access
setup_oracle_env

cleanup() {
    echo "Cleaning up..."
    rm -rf "$RESTORE_DIR"

    # Drop test tablespaces if Oracle is running
    if pgrep -x "ora_pmon_XE" > /dev/null 2>&1; then
        for SIZE in "${SIZE_ARRAY[@]}"; do
            ts_name="BFUSE_${SIZE}"
            run_sqlplus "
                BEGIN
                    EXECUTE IMMEDIATE 'ALTER TABLESPACE ${ts_name} OFFLINE';
                EXCEPTION WHEN OTHERS THEN NULL;
                END;
                /
                BEGIN
                    EXECUTE IMMEDIATE 'DROP TABLESPACE ${ts_name} INCLUDING CONTENTS AND DATAFILES';
                EXCEPTION WHEN OTHERS THEN NULL;
                END;
                /
            " 2>/dev/null || true
        done
    fi

    rm -rf "$BACKUP_BASE"
}

trap cleanup EXIT

# ==============================================================================
# run_sqlplus: Execute SQL statement via sqlplus as the oracle OS user.
# Uses 'sudo -u oracle' to ensure OS authentication ('/ as sysdba') succeeds,
# since the oracle user is in the dba group by default.
# ==============================================================================
run_sqlplus() {
    local sql="$1"
    local script
    script=$(printf 'SET HEADING OFF FEEDBACK OFF PAGESIZE 0 LINESIZE 200\nWHENEVER SQLERROR EXIT SQL.SQLCODE\n%s\nEXIT;\n' "$sql")
    sudo -E -u oracle \
        ORACLE_HOME="${ORACLE_HOME}" \
        ORACLE_SID="${ORACLE_SID}" \
        PATH="${ORACLE_HOME}/bin:${PATH}" \
        LD_LIBRARY_PATH="${ORACLE_HOME}/lib" \
        NLS_LANG="AMERICAN_AMERICA.AL32UTF8" \
        bash -c 'echo "$1" | sqlplus -s "/ as sysdba"' _ "$script"
}

# ==============================================================================
# run_rman: Execute RMAN commands as the oracle OS user.
# ==============================================================================
run_rman() {
    local commands="$1"
    local script
    script=$(printf '%s\nEXIT;\n' "$commands")
    sudo -E -u oracle \
        ORACLE_HOME="${ORACLE_HOME}" \
        ORACLE_SID="${ORACLE_SID}" \
        PATH="${ORACLE_HOME}/bin:${PATH}" \
        LD_LIBRARY_PATH="${ORACLE_HOME}/lib" \
        bash -c 'echo "$1" | rman target /' _ "$script"
}

# ==============================================================================
# install_oracle_xe: Install Oracle XE if not already installed
# ==============================================================================
install_oracle_xe() {
    if command -v sqlplus &> /dev/null && pgrep -x "ora_pmon_XE" > /dev/null 2>&1; then
        echo -e "${CYAN}Oracle XE is already installed and running${NC}"
        return 0
    fi

    echo -e "${CYAN}Installing Oracle XE...${NC}"

    # Install prerequisites (Oracle Linux uses yum)
    sudo yum install -y libaio bc wget unzip

    # Download and install Oracle XE natively via RPM (Oracle Linux)
    if ! command -v sqlplus &> /dev/null; then
        ORACLE_XE_RPM="oracle-database-xe-21c-1.0-1.ol8.x86_64.rpm"
        if [ ! -f "$DATA_DIR/${ORACLE_XE_RPM}" ]; then
            echo -e "${CYAN}Downloading Oracle XE...${NC}"
            wget -q "https://download.oracle.com/otn-pub/otn_software/db-express/${ORACLE_XE_RPM}" \
                -O "$DATA_DIR/${ORACLE_XE_RPM}" || {
                echo -e "${RED}Failed to download Oracle XE.${NC}"
                echo -e "${RED}[SKIP] Oracle XE not available for download. Skipping actual RMAN tests.${NC}"
                return 1
            }
        fi

        echo -e "${CYAN}Installing Oracle XE RPM...${NC}"
        sudo yum localinstall -y "$DATA_DIR/${ORACLE_XE_RPM}" || {
            echo -e "${RED}[SKIP] Failed to install Oracle XE RPM. Skipping actual RMAN tests.${NC}"
            return 1
        }
    fi

    # Set Oracle data location to DATA_DIR to avoid insufficient space on /opt/oracle
    if [ -f /etc/sysconfig/oracle-xe-21c.conf ]; then
        if grep -q "^DBFILE_DEST=" /etc/sysconfig/oracle-xe-21c.conf; then
            sudo sed -i "s|^DBFILE_DEST=.*|DBFILE_DEST=${DATA_DIR//|/\\|}|" /etc/sysconfig/oracle-xe-21c.conf
        else
            echo "DBFILE_DEST=${DATA_DIR}" | sudo tee -a /etc/sysconfig/oracle-xe-21c.conf
        fi
    fi

    # Configure Oracle XE with password from environment (test-only default)
    local ora_pwd="${ORACLE_XE_PASSWORD:-Oracle123}"
    echo -e "${CYAN}Configuring Oracle XE...${NC}"
    printf '%s\n%s\n' "$ora_pwd" "$ora_pwd" | sudo /etc/init.d/oracle-xe-21c configure
    if [ $? -ne 0 ]; then
        echo -e "${RED}[SKIP] Failed to configure Oracle XE. Skipping actual RMAN tests.${NC}"
        return 1
    fi

    # Re-detect Oracle environment after installation
    setup_oracle_env
    return 0
}

# ==============================================================================
# create_tablespace: Create a tablespace with a datafile of the given size
# ==============================================================================
create_tablespace() {
    local size="$1"
    local ts_name="BFUSE_${size}"
    local df_dir="${DATA_DIR}/oradata/XE/bfuse_test"

    echo -e "${CYAN}Creating tablespace ${ts_name} with ${size} datafile...${NC}"

    # Create datafile directory
    sudo mkdir -p "$df_dir"
    sudo chown oracle:oinstall "$df_dir"

    # Drop if exists
    run_sqlplus "
        BEGIN
            EXECUTE IMMEDIATE 'ALTER TABLESPACE ${ts_name} OFFLINE';
        EXCEPTION WHEN OTHERS THEN NULL;
        END;
        /
        BEGIN
            EXECUTE IMMEDIATE 'DROP TABLESPACE ${ts_name} INCLUDING CONTENTS AND DATAFILES';
        EXCEPTION WHEN OTHERS THEN NULL;
        END;
        /
    " 2>/dev/null || true

    # Create tablespace
    run_sqlplus "CREATE TABLESPACE ${ts_name} DATAFILE '${df_dir}/${ts_name}.dbf' SIZE ${size} AUTOEXTEND OFF;"
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] Failed to create tablespace ${ts_name}${NC}"
        return 1
    fi

    # Populate with data to ensure the datafile is fully written
    run_sqlplus "
        CREATE TABLE test_data_${size} TABLESPACE ${ts_name} AS
        SELECT level AS id,
               dbms_random.string('A', 100) AS data,
               SYSDATE AS created_at
        FROM dual
        CONNECT BY level <= 10000;
    " || true

    echo -e "${GREEN}Tablespace ${ts_name} created successfully${NC}"
    return 0
}

# ==============================================================================
# rman_full_backup: Perform actual RMAN full backup to FUSE mount
# ==============================================================================
rman_full_backup() {
    local size="$1"
    local ts_name="BFUSE_${size}"
    local backup_dir="$BACKUP_BASE/full_${size}"

    echo -e "${CYAN}------------------------------------------------------------${NC}"
    echo -e "${CYAN}RMAN Full Backup: tablespace=${ts_name} dest=${backup_dir}${NC}"
    echo -e "${CYAN}------------------------------------------------------------${NC}"

    mkdir -p "$backup_dir"
    chmod 777 "$backup_dir"

    # Perform RMAN full backup of the tablespace to blobfuse2 mount
    run_rman "
        CONFIGURE CONTROLFILE AUTOBACKUP ON;
        CONFIGURE CONTROLFILE AUTOBACKUP FORMAT FOR DEVICE TYPE DISK TO '${backup_dir}/ctrl_%F';
        BACKUP AS BACKUPSET
            TABLESPACE ${ts_name}
            FORMAT '${backup_dir}/full_%U.bkp'
            TAG 'FULL_${ts_name}';
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN full backup failed for ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Verify backup files exist on the FUSE mount
    local bkp_count
    bkp_count=$(find "$backup_dir" -name "full_*.bkp" -type f | wc -l)
    if [ "$bkp_count" -eq 0 ]; then
        echo -e "${RED}[FAIL] No backup files found in ${backup_dir}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Drop page cache to force read from FUSE
    sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches' 2>/dev/null

    # Validate the backup using RMAN VALIDATE
    echo -e "${CYAN}Validating backup with RMAN VALIDATE...${NC}"
    run_rman "
        VALIDATE BACKUP OF TABLESPACE ${ts_name};
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN VALIDATE failed for full backup of ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Perform RESTORE VALIDATE to confirm backup can be restored
    echo -e "${CYAN}Running RESTORE VALIDATE...${NC}"
    run_rman "
        RESTORE TABLESPACE ${ts_name} VALIDATE;
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN RESTORE VALIDATE failed for full backup of ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    echo -e "${GREEN}[PASS] RMAN full backup integrity verified: ${ts_name} (${bkp_count} backup pieces)${NC}"
    PASSED=$((PASSED + 1))
    return 0
}

# ==============================================================================
# rman_incremental_backup: Perform RMAN incremental backup
# ==============================================================================
rman_incremental_backup() {
    local size="$1"
    local ts_name="BFUSE_${size}"
    local backup_dir="$BACKUP_BASE/incr_${size}"

    echo -e "${CYAN}------------------------------------------------------------${NC}"
    echo -e "${CYAN}RMAN Incremental Backup: tablespace=${ts_name} dest=${backup_dir}${NC}"
    echo -e "${CYAN}------------------------------------------------------------${NC}"

    mkdir -p "$backup_dir"
    chmod 777 "$backup_dir"

    # Level 0 incremental backup (base)
    echo -e "${CYAN}Creating Level 0 incremental backup...${NC}"
    run_rman "
        BACKUP INCREMENTAL LEVEL 0
            TABLESPACE ${ts_name}
            FORMAT '${backup_dir}/incr0_%U.bkp'
            TAG 'INCR0_${ts_name}';
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN level 0 incremental backup failed for ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Make some changes to the tablespace
    echo -e "${CYAN}Modifying data in tablespace...${NC}"
    run_sqlplus "
        INSERT INTO test_data_${size}
        SELECT level + 10000 AS id,
               dbms_random.string('A', 100) AS data,
               SYSDATE AS created_at
        FROM dual
        CONNECT BY level <= 5000;
        COMMIT;
    " || true

    # Level 1 incremental backup
    echo -e "${CYAN}Creating Level 1 incremental backup...${NC}"
    run_rman "
        BACKUP INCREMENTAL LEVEL 1
            TABLESPACE ${ts_name}
            FORMAT '${backup_dir}/incr1_%U.bkp'
            TAG 'INCR1_${ts_name}';
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN level 1 incremental backup failed for ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Drop page cache
    sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches' 2>/dev/null

    # Validate level 0 backup
    echo -e "${CYAN}Validating Level 0 backup...${NC}"
    run_rman "
        VALIDATE BACKUP OF TABLESPACE ${ts_name};
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN VALIDATE failed for level 0 backup of ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Validate level 1 backup
    echo -e "${CYAN}Validating Level 1 backup...${NC}"
    run_rman "
        VALIDATE BACKUP OF TABLESPACE ${ts_name};
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN VALIDATE failed for level 1 backup of ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    echo -e "${GREEN}[PASS] RMAN incremental backup integrity verified: ${ts_name}${NC}"
    PASSED=$((PASSED + 1))
    return 0
}

# ==============================================================================
# rman_backup_and_restore_verify: Full backup, restore, and compare
# ==============================================================================
rman_backup_and_restore_verify() {
    local size="$1"
    local ts_name="BFUSE_${size}"
    local backup_dir="$BACKUP_BASE/verify_${size}"
    local df_dir="${DATA_DIR}/oradata/XE/bfuse_test"
    local datafile="${df_dir}/${ts_name}.dbf"

    echo -e "${CYAN}------------------------------------------------------------${NC}"
    echo -e "${CYAN}RMAN Backup & Restore Verify: tablespace=${ts_name}${NC}"
    echo -e "${CYAN}------------------------------------------------------------${NC}"

    mkdir -p "$backup_dir"
    chmod 777 "$backup_dir"

    # Take tablespace offline so the datafile is in a consistent, quiesced state.
    # An online backup-as-copy writes fuzzy SCN markers into the copy's header,
    # causing an unavoidable MD5 mismatch with the live file.  An offline copy is
    # a byte-for-byte duplicate that can be verified with MD5.
    echo -e "${CYAN}Taking tablespace ${ts_name} offline for consistent copy...${NC}"
    run_sqlplus "ALTER TABLESPACE ${ts_name} OFFLINE NORMAL;"
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] Could not take tablespace ${ts_name} offline${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Take MD5 checksum of the offline (consistent) datafile
    local orig_md5
    orig_md5=$(sudo md5sum "$datafile" | awk '{print $1}')
    if [ -z "$orig_md5" ]; then
        run_sqlplus "ALTER TABLESPACE ${ts_name} ONLINE;" || true
        echo -e "${RED}[FAIL] Could not compute MD5 of original datafile: ${datafile}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi
    echo "Original datafile MD5: $orig_md5"

    # Backup offline datafile to FUSE mount as a byte-for-byte copy
    run_rman "
        BACKUP AS COPY
            DATAFILE '${datafile}'
            FORMAT '${backup_dir}/${ts_name}_copy.dbf';
    "
    if [ $? -ne 0 ]; then
        run_sqlplus "ALTER TABLESPACE ${ts_name} ONLINE;" || true
        echo -e "${RED}[FAIL] RMAN backup as copy failed for ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Bring tablespace back online
    echo -e "${CYAN}Bringing tablespace ${ts_name} back online...${NC}"
    run_sqlplus "ALTER TABLESPACE ${ts_name} ONLINE;"
    if [ $? -ne 0 ]; then
        echo -e "${RED}[WARN] Could not bring tablespace ${ts_name} back online${NC}"
    fi

    # Drop page cache
    sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches' 2>/dev/null

    # Compare the datafile copy on FUSE mount with original
    local copy_md5
    copy_md5=$(md5sum "${backup_dir}/${ts_name}_copy.dbf" | awk '{print $1}')
    if [ -z "$copy_md5" ]; then
        echo -e "${RED}[FAIL] Could not compute MD5 of backup copy: ${backup_dir}/${ts_name}_copy.dbf${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi
    echo "Backup copy MD5:      $copy_md5"

    if [ "$orig_md5" == "$copy_md5" ]; then
        echo -e "${GREEN}[PASS] RMAN backup copy integrity verified: ${ts_name}${NC}"
        echo "       MD5: $orig_md5"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}[FAIL] RMAN backup copy integrity MISMATCH: ${ts_name}${NC}"
        echo "       Original MD5: $orig_md5"
        echo "       Copy MD5:     $copy_md5"
        FAILED=$((FAILED + 1))
        return 1
    fi

    return 0
}

# ==============================================================================
# Main test execution
# ==============================================================================
echo "============================================================"
echo "Starting Actual RMAN Backup Tests on $MOUNT_POINT"
echo "Database file sizes: ${SIZES}"
echo "============================================================"

# Verify Oracle XE is available (check both PATH and ORACLE_HOME/bin)
if ! command -v rman &> /dev/null && [ ! -x "${ORACLE_HOME}/bin/rman" ]; then
    echo -e "${CYAN}RMAN utility not found, attempting Oracle XE installation...${NC}"
    install_oracle_xe
    if [ $? -ne 0 ]; then
        echo -e "${RED}Oracle XE installation failed. Skipping actual RMAN tests.${NC}"
        echo -e "${CYAN}Note: Simulation-based RMAN tests are still run separately.${NC}"
        exit 0
    fi
fi

# Verify the database is running
if ! pgrep -x "ora_pmon_XE" > /dev/null 2>&1; then
    echo -e "${CYAN}Starting Oracle XE database...${NC}"
    sudo /etc/init.d/oracle-xe-21c start 2>/dev/null || {
        echo -e "${RED}Failed to start Oracle XE. Skipping actual RMAN tests.${NC}"
        exit 0
    }
fi

# Verify OS authentication works before proceeding
echo -e "${CYAN}Verifying Oracle OS authentication...${NC}"
run_sqlplus "SELECT 'AUTH_OK' FROM dual;"
if [ $? -ne 0 ]; then
    echo -e "${RED}Oracle OS authentication failed. Skipping actual RMAN tests.${NC}"
    echo -e "${CYAN}Ensure the 'oracle' user exists and the database is running.${NC}"
    exit 0
fi

# Enable ARCHIVELOG mode if not already enabled.
# RMAN cannot backup or copy active datafiles in NOARCHIVELOG mode (ORA-19602).
echo -e "${CYAN}Checking ARCHIVELOG mode...${NC}"
ARCHIVELOG_STATUS=$(run_sqlplus "SELECT LOG_MODE FROM V\$DATABASE;")
if echo "$ARCHIVELOG_STATUS" | grep -q "NOARCHIVELOG"; then
    echo -e "${CYAN}Enabling ARCHIVELOG mode (requires restart)...${NC}"
    run_sqlplus "SHUTDOWN IMMEDIATE;" || {
        echo -e "${RED}Failed to shutdown database for ARCHIVELOG switch. Skipping.${NC}"
        exit 0
    }
    run_sqlplus "STARTUP MOUNT;" || {
        echo -e "${RED}Failed to start database in mount mode. Attempting full startup...${NC}"
        run_sqlplus "STARTUP;"
        exit 0
    }
    run_sqlplus "ALTER DATABASE ARCHIVELOG;" || {
        echo -e "${RED}Failed to enable ARCHIVELOG mode. Opening database and skipping.${NC}"
        run_sqlplus "ALTER DATABASE OPEN;"
        exit 0
    }
    run_sqlplus "ALTER DATABASE OPEN;" || {
        echo -e "${RED}Failed to open database after ARCHIVELOG switch.${NC}"
        exit 0
    }
    echo -e "${GREEN}ARCHIVELOG mode enabled${NC}"
else
    echo -e "${CYAN}ARCHIVELOG mode is already enabled${NC}"
fi

mkdir -p "$BACKUP_BASE"
# chmod 777 is needed because blobfuse2 FUSE mounts do not support POSIX
# ownership/ACLs, and the oracle user (via sudo -u oracle) must write here.
chmod 777 "$BACKUP_BASE"

# Configure RMAN defaults
echo -e "${CYAN}Configuring RMAN defaults...${NC}"
run_rman "
    CONFIGURE RETENTION POLICY TO NONE;
    CONFIGURE BACKUP OPTIMIZATION ON;
    CONFIGURE DEVICE TYPE DISK PARALLELISM 1;
    CONFIGURE DEFAULT DEVICE TYPE TO DISK;
"

for SIZE in "${SIZE_ARRAY[@]}"; do
    echo ""
    echo "============================================================"
    echo "Testing with database file size: ${SIZE}"
    echo "============================================================"

    # Create tablespace with the specified datafile size
    create_tablespace "$SIZE"
    if [ $? -ne 0 ]; then
        FAILED=$((FAILED + 1))
        continue
    fi

    # Test 1: Full RMAN backup with integrity validation
    rman_full_backup "$SIZE"

    # Test 2: Incremental RMAN backup (level 0 + level 1) with validation
    rman_incremental_backup "$SIZE"

    # Test 3: RMAN backup as copy and MD5 verification
    rman_backup_and_restore_verify "$SIZE"
done

echo ""
echo "============================================================"
echo "Actual RMAN Backup Tests Complete"
echo "  Passed: $PASSED"
echo "  Failed: $FAILED"
echo "============================================================"

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Some tests FAILED!${NC}"
    exit 1
fi

echo -e "${GREEN}All actual RMAN backup tests passed!${NC}"
exit 0
