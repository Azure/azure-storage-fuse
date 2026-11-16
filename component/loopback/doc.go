// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

/*
Package loopback provides loopbackfs, a storage component that emulates Azure Storage on a local directory. It
implements internal.Component the way the azstorage component does, so that pipelines can be tested and mounted without
a storage account, for example to reproduce a bug:

	components:
	  - libfuse
	  - block_cache        # or file_cache, xload, attr_cache, ...
	  - loopbackfs
	loopbackfs:
	  path: /tmp/loopback  # backing directory, created if missing

Every object of the emulated container is a file, directory or symbolic link with the same path in the backing
directory. Objects can be created, inspected and changed directly in the backing directory.

# Semantics

loopbackfs returns what azstorage returns for the same operation:

  - Errors are bare errno values, which callers compare with ==, such as syscall.ENOENT for an object that does not
    exist, including an object below a file.
  - Handles hold no resources, so callers can drop them. Reads and writes through a handle address the object by
    name: an object renamed or deleted while open cannot be reached through the handle anymore.
  - Uploads (CopyFromFile) replace an object at once, so readers see either the old or the new content.
  - Attributes report the last modification time as the access, change and creation time too, and mark the mode as
    default. A rename updates the modification time, and every change made through the component changes the ETag.
  - StreamDir returns pages of entries in the order of Azure Storage, with continuation tokens.
  - Objects are created along with their missing parents.

Directories are real, as in an account with a hierarchical namespace: CreateDir fails with EEXIST for an existing
directory and DeleteDir fails with ENOTEMPTY for a directory that has entries. User-defined metadata is not stored.

Uploads in progress are kept in the .loopbackfs directory of the backing directory. Operations of the component never
list it or reach it. It can be deleted when no mount uses the backing directory.

# Options

	loopbackfs:
	  path: <backing directory>
	  list-page-size: <most entries per StreamDir page, 1 to 5000. Default - 5000>
	  content-md5: true|false <report the MD5 of file contents in GetAttr and listings. Default - false>
	  faults: <list of faults, see below>

All options but path can change while mounted: they are applied again when the configuration file changes.

# Faults

loopbackfs can inject errors and delays into its operations, to emulate failures and slowness of Azure Storage:

	loopbackfs:
	  path: /tmp/loopback
	  faults:
	    - op: ReadInBuffer   # operation, named after the component method. * or none for all
	      path: "dir/*.bin"  # path.Match pattern of the objects, without a leading slash. None for all
	      error: EIO         # errno to fail with, such as ENOENT, EACCES or ETIMEDOUT. None to only delay
	      delay-ms: 200      # how long to wait before running or failing the call
	      skip: 1            # number of matching calls to let through first
	      count: 2           # number of calls to apply the fault to after the skipped ones. 0 or none for all

Tests inject faults with LoopbackFS.InjectFault.
*/
package loopback
