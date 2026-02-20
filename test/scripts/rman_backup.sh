#!/bin/bash

# rman_backup.sh
# Performs actual Oracle RMAN backup workloads on a FUSE-mounted filesystem.
# Installs Oracle XE, creates tablespaces with datafiles of various sizes,
# performs full and incremental backups using the rman utility, and validates
# data integrity.
#
# Usage: ./rman_backup.sh /path/to/mountpoint [sizes]
#   sizes: comma-separated list of datafile sizes (default: 10M,100M,1G,10G)

set -uo pipefail

MOUNT_POINT="$1"
SIZES="${2:-10M,100M,1G,10G}"

if [ -z "$MOUNT_POINT" ]; then
    echo "Usage: $0 /path/to/mountpoint [sizes]"
    echo "  sizes: comma-separated list (e.g., 10M,100M,1G,10G)"
    exit 1
fi

if [ ! -d "$MOUNT_POINT" ]; then
    echo "Error: Directory $MOUNT_POINT does not exist."
    exit 1
fi

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

BACKUP_BASE="$MOUNT_POINT/rman_backup"
RESTORE_DIR="/tmp/rman_restore_$$"
PASSED=0
FAILED=0

IFS=',' read -ra SIZE_ARRAY <<< "$SIZES"

cleanup() {
    echo "Cleaning up..."
    rm -rf "$RESTORE_DIR"

    # Drop test tablespaces if Oracle is running
    if pgrep -x "ora_pmon_XE" > /dev/null 2>&1; then
        for SIZE in "${SIZE_ARRAY[@]}"; do
            local ts_name="BFUSE_${SIZE}"
            sqlplus -s / as sysdba <<EOF 2>/dev/null || true
ALTER TABLESPACE ${ts_name} OFFLINE;
DROP TABLESPACE ${ts_name} INCLUDING CONTENTS AND DATAFILES;
EOF
        done
    fi

    rm -rf "$BACKUP_BASE"
}

trap cleanup EXIT

# ==============================================================================
# install_oracle_xe: Install Oracle XE if not already installed
# ==============================================================================
install_oracle_xe() {
    if command -v sqlplus &> /dev/null && pgrep -x "ora_pmon_XE" > /dev/null 2>&1; then
        echo -e "${CYAN}Oracle XE is already installed and running${NC}"
        return 0
    fi

    echo -e "${CYAN}Installing Oracle XE...${NC}"

    # Install prerequisites
    sudo apt-get update -y
    sudo apt-get install -y libaio1 unzip wget bc

    # Download and install Oracle XE
    # Use the Oracle XE RPM via alien for Ubuntu
    if ! command -v sqlplus &> /dev/null; then
        sudo apt-get install -y alien

        # Download Oracle XE 21c
        ORACLE_XE_RPM="oracle-database-xe-21c-1.0-1.ol8.x86_64.rpm"
        if [ ! -f "/tmp/${ORACLE_XE_RPM}" ]; then
            echo -e "${CYAN}Downloading Oracle XE...${NC}"
            wget -q "https://download.oracle.com/otn-pub/otn_software/db-express/${ORACLE_XE_RPM}" \
                -O "/tmp/${ORACLE_XE_RPM}" || {
                echo -e "${RED}Failed to download Oracle XE. Trying alternative mirror...${NC}"
                # If direct download fails, the pipeline agents should have Oracle XE pre-installed
                echo -e "${RED}[SKIP] Oracle XE not available for download. Skipping actual RMAN tests.${NC}"
                return 1
            }
        fi

        echo -e "${CYAN}Converting RPM to DEB and installing...${NC}"
        cd /tmp
        sudo alien --to-deb --scripts "${ORACLE_XE_RPM}" || {
            echo -e "${RED}[SKIP] Failed to convert Oracle XE package. Skipping actual RMAN tests.${NC}"
            return 1
        }
        sudo dpkg -i oracle-database-xe-21c*.deb || {
            echo -e "${RED}[SKIP] Failed to install Oracle XE. Skipping actual RMAN tests.${NC}"
            return 1
        }
    fi

    # Configure Oracle XE with default password
    echo -e "${CYAN}Configuring Oracle XE...${NC}"
    printf 'Oracle123\nOracle123\n' | sudo /etc/init.d/oracle-xe-21c configure
    if [ $? -ne 0 ]; then
        echo -e "${RED}[SKIP] Failed to configure Oracle XE. Skipping actual RMAN tests.${NC}"
        return 1
    fi

    # Set environment
    setup_oracle_env
    return 0
}

# ==============================================================================
# setup_oracle_env: Set Oracle environment variables
# ==============================================================================
setup_oracle_env() {
    export ORACLE_HOME="${ORACLE_HOME:-/opt/oracle/product/21c/dbhomeXE}"
    export ORACLE_SID="${ORACLE_SID:-XE}"
    export PATH="$ORACLE_HOME/bin:$PATH"
    export LD_LIBRARY_PATH="$ORACLE_HOME/lib:${LD_LIBRARY_PATH:-}"
    export NLS_LANG="AMERICAN_AMERICA.AL32UTF8"
}

# ==============================================================================
# run_sqlplus: Execute SQL statement via sqlplus
# ==============================================================================
run_sqlplus() {
    local sql="$1"
    sqlplus -s / as sysdba <<EOF
SET HEADING OFF FEEDBACK OFF PAGESIZE 0 LINESIZE 200
WHENEVER SQLERROR EXIT SQL.SQLCODE
${sql}
EXIT;
EOF
}

# ==============================================================================
# run_rman: Execute RMAN commands
# ==============================================================================
run_rman() {
    local commands="$1"
    rman target / <<EOF
${commands}
EXIT;
EOF
}

# ==============================================================================
# create_tablespace: Create a tablespace with a datafile of the given size
# ==============================================================================
create_tablespace() {
    local size="$1"
    local ts_name="BFUSE_${size}"
    local df_dir="/opt/oracle/oradata/XE/bfuse_test"

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
        VALIDATE BACKUPSET TAG 'FULL_${ts_name}';
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
        VALIDATE BACKUPSET TAG 'INCR0_${ts_name}';
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN VALIDATE failed for level 0 backup of ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Validate level 1 backup
    echo -e "${CYAN}Validating Level 1 backup...${NC}"
    run_rman "
        VALIDATE BACKUPSET TAG 'INCR1_${ts_name}';
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
    local df_dir="/opt/oracle/oradata/XE/bfuse_test"
    local datafile="${df_dir}/${ts_name}.dbf"

    echo -e "${CYAN}------------------------------------------------------------${NC}"
    echo -e "${CYAN}RMAN Backup & Restore Verify: tablespace=${ts_name}${NC}"
    echo -e "${CYAN}------------------------------------------------------------${NC}"

    mkdir -p "$backup_dir"

    # Take MD5 checksum of original datafile
    local orig_md5
    orig_md5=$(md5sum "$datafile" 2>/dev/null | awk '{print $1}')
    echo "Original datafile MD5: $orig_md5"

    # Backup tablespace to FUSE mount
    run_rman "
        BACKUP AS COPY
            DATAFILE '${datafile}'
            FORMAT '${backup_dir}/${ts_name}_copy.dbf';
    "
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] RMAN backup as copy failed for ${ts_name}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Drop page cache
    sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches' 2>/dev/null

    # Compare the datafile copy on FUSE mount with original
    local copy_md5
    copy_md5=$(md5sum "${backup_dir}/${ts_name}_copy.dbf" 2>/dev/null | awk '{print $1}')
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

# Set up Oracle environment
setup_oracle_env

# Verify Oracle XE is available
if ! command -v rman &> /dev/null; then
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

mkdir -p "$BACKUP_BASE"

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
