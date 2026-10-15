---
description: "Use when changing component/azstorage: Azure Blob and ADLS Gen2 SDK calls, block_blob.go and datalake.go, authentication modes, storage error mapping, SDK logging, azstorage tests."
applyTo: "component/azstorage/**"
---
# azstorage component

- `BlockBlob` (`block_blob.go`, flat namespace) and `Datalake` (`datalake.go`, hierarchical namespace) both implement `AzConnection` in `connection.go`. `Datalake` keeps a `BlockBlob` field and delegates reads, writes, block staging and listing to it, but its own create, rename, delete and permission operations differ. Check whether a fix in one file also needs the other.
- SDK errors are classified by `storeBlobErrToErr` and `storeDatalakeErrToErr` in `utils.go`, and the callers turn the result into `syscall` errors (`ENOENT`, `EEXIST`, `EINVAL`, ...) that libfuse returns to applications. Map a new storage error code there instead of returning the raw SDK error, which becomes `EIO`.
- Listings are paged, and a page can be empty but still have a continuation token, so only a listing that ran to the end without error proves that a directory is empty, as in `AzStorage.IsDirEmpty`. `Datalake.List` uses `BlockBlob.List`, which also returns virtual directories (`BlobPrefixes`) that have no marker blob.
- `GetCommittedBlockList` returns `(nil, nil)` for a blob without committed blocks: an empty blob, or one uploaded in a single Put Blob request. Callers must handle a nil list for a file that has data.
- Each authentication mode has its own file (`azauthkey.go`, `azauthsas.go`, `azauthmsi.go`, `azauthspn.go`, `azauthcli.go`, `azauthWorkloadIdentity.go`). Options are parsed and validated in `config.go`; a new option also follows the [add-config-option skill](../skills/add-config-option/SKILL.md).
- SDK request logging redacts every header and query parameter except those in `allowedHeaders` and `allowedQueryParams` in `azstorage_constants.go`. Never add a header or parameter that can carry a secret, such as `sig`, `Authorization` or `x-ms-encryption-key`.
- `TestAuthTestSuite`, `TestBlockBlob` and `TestDatalake` run against the real storage account in `~/azuretest.json` and are skipped by `make test`; run them only when asked, with `make test-azure`. Test new behavior offline with an `httptest.Server` that plays the storage service, as in `datalake_create_test.go` and `auth_validation_test.go`.
