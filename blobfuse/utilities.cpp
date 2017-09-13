#include "blobfuse.h"

int map_errno(int error)
{
	auto mapping = error_mapping.find(error);
	if (mapping == error_mapping.end())
	{
		return error;
	}
	else
	{
		return mapping->second;
	}
}

std::string prepend_mnt_path_string(const std::string path)
{
	return str_options.tmpPath + "/root" + path;
}

void ensure_files_directory_exists(const std::string file_path)
{
	char *pp;
	char *slash;
	int status;
	char *copypath = strdup(file_path.c_str());

	status = 0;
	pp = copypath;
	while (status == 0 && (slash = strchr(pp, '/')) != 0)
	{
		if (slash != pp)
		{
			*slash = '\0';
			if (AZS_PRINT)
			{
				fprintf(stdout, "Making directory %s\n", copypath);
			}
			struct stat st;
			if (stat(copypath, &st) != 0)
			{
				status = mkdir(copypath, 0777);
			}
			*slash = '/';
		}
		pp = slash + 1;
	}
	free(copypath);
}

std::vector<list_blobs_hierarchical_item> list_all_blobs_hierarchical(std::string container, std::string delimiter, std::string prefix)
{
	std::vector<list_blobs_hierarchical_item> results;

	std::string continuation;

	bool success = false;
	int failcount = 0;
	do
	{
		if (AZS_PRINT)
		{
			fprintf(stdout, "About to call list_blobs_hierarchial.  Container = %s, delimiter = %s, continuation = %s, prefix = %s\n", container.c_str(), delimiter.c_str(), continuation.c_str(), prefix.c_str());
		}

		errno = 0;
		list_blobs_hierarchical_response response = azure_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation, prefix);
		if (errno == 0)
		{
			success = true;
			failcount = 0;
			if (AZS_PRINT)
			{
				fprintf(stdout, "results count = %lu\n", response.blobs.size());
				fprintf(stdout, "next_marker = %s\n", response.next_marker.c_str());
			}
			continuation = response.next_marker;
			results.insert(results.end(), response.blobs.begin(), response.blobs.end());
		}
		else
		{
			failcount++;
			success = false;
		}
	} while (((continuation.size() > 0) || !success) && (failcount < 20));

	return results;
}

bool list_one_blob_hierarchical(std::string container, std::string delimiter, std::string prefix)
{
	std::string continuation;
	bool success = false;
	int failcount = 0;

	do
	{
		errno = 0;
		list_blobs_hierarchical_response response = azure_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation, prefix);
		if (errno == 0)
		{
			success = true;
			failcount = 0;
			continuation = response.next_marker;
			if (response.blobs.size() > 0)
			{
				return true;
			}
		}
		else
		{
			success = false;
			failcount++; //TODO: use to set errno.
		}
	} while ((continuation.size() > 0) && !success && (failcount < 20));

	return false;
}

// Returns:
// 0 if there's nothing there (the directory does not exist)
// 1 is there's exactly one blob, and it's the ".directory" blob
// 2 otherwise (the directory exists and is not empty.)
int is_directory_empty(std::string container, std::string delimiter, std::string prefix)
{
	std::string continuation;
	bool success = false;
	int failcount = 0;
	bool dirBlobFound = false;
	do
	{
		errno = 0;
		list_blobs_hierarchical_response response = azure_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation, prefix);
		if (errno == 0)
		{
			success = true;
			failcount = 0;
			continuation = response.next_marker;
			if (response.blobs.size() > 1)
			{
				return 2;
			}
			if (response.blobs.size() == 1)
			{
				if ((!dirBlobFound) && (!response.blobs[0].is_directory) && (response.blobs[0].name.size() > directorySignifier.size()) && (response.blobs[0].name.compare(response.blobs[0].name.size() - (directorySignifier.size() + 1), directorySignifier.size(), directorySignifier)))
				{
					dirBlobFound = true;
				}
				else
				{
					return 2;
				}
			}
		}
		else
		{
			success = false;
			failcount++; //TODO: use to set errno.
		}
	} while ((continuation.size() > 0) && !success && (failcount < 20));

	return dirBlobFound ? 1 : 0;
}


int azs_getattr(const char *path, struct stat *stbuf)
{
	if (AZS_PRINT)
	{
		fprintf(stdout, "azs_getattr called, Path requested = %s\n", path);
	}
	// If we're at the root, we know it's a directory
	if (strlen(path) == 1)
	{
		stbuf->st_mode = S_IFDIR | 0777; // TODO: proper access control.
		stbuf->st_nlink = 2; // Directories should have a hard-link count of 2 + (# child directories).  We don't have that count, though, so we jsut use 2 for now.  TODO: Evaluate if we could keep this accurate or not.
		return 0;
	}

	// Check and see if the file/directory exists locally (because it's being buffered.)  If so, skip the call to Storage.
	std::string pathString(path);
	const char * mntPath;
	std::string mntPathString = prepend_mnt_path_string(pathString);
	mntPath = mntPathString.c_str();

	int res;
	int acc = access(mntPathString.c_str(), F_OK);
	if (AZS_PRINT)
	{
		fprintf(stdout, "accessing mntPath = %s returned %d\n", mntPathString.c_str(), acc);
	}  if (acc != -1 )
	{
		//(void) fi;
		res = lstat(mntPathString.c_str(), stbuf);
		if (AZS_PRINT)
		{
			printf("LSTAT res = %d, errno = %d, ENOENT = %d\n", res, errno, ENOENT);
		}
		if (res == -1)
			return -errno;
		return 0;
	}

	std::string containerNameStr(options.containerName);
	std::string blobNameStr(&(path[1]));

	errno = 0;
	auto blob_property = azure_blob_client_wrapper->get_blob_property(containerNameStr, blobNameStr);

	int result = 1;
	if ((errno == 0) && blob_property.valid())
	{
		if (AZS_PRINT)
		{
			fprintf(stdout, "Blob found!  Name = %s\n", path);
		}
		stbuf->st_mode = S_IFREG | 0777; // Regular file (not a directory)
		stbuf->st_nlink = 1;
		stbuf->st_size = blob_property.size;
		return 0;
	}
	else if (errno == 0 && !blob_property.valid())
	{
		// Check to see if it's a directory, instead of a file
		blobNameStr.push_back('/');

		errno = 0;
		bool dirExists = list_one_blob_hierarchical(containerNameStr, "/", blobNameStr);

		if (errno != 0)
		{
			if (AZS_PRINT)
			{
				fprintf(stdout, "Tried to find dir %s, but received errno = %d\n", path, errno);
			}
			return 0 - map_errno(errno);
		}
		if (dirExists)
		{
			if (AZS_PRINT)
			{
				fprintf(stdout, "Directory %s found!\n", blobNameStr.c_str());
			}
			stbuf->st_mode = S_IFDIR | 0777;
			stbuf->st_nlink = 2;
			return 0;
		}
		else
		{
			return -(ENOENT); // -2 = Entity does not exist.
		}
	}
	else
	{
		return 0 - map_errno(errno);
	}
}

int rm(const char *fpath, const struct stat *sb, int tflag, struct FTW *ftwbuf)
{
	if (tflag == FTW_DP)
	{
		errno = 0;
		int ret = rmdir(fpath);
		return ret;
	}
	else
	{
		errno = 0;
		int ret = unlink(fpath);
		return ret;
	}
}

void azs_destroy(void *private_data)
{
	std::string rootPath(str_options.tmpPath + "/root");
	char *cstr = (char *)malloc(rootPath.size() + 1);
	memcpy(cstr, rootPath.c_str(), rootPath.size());
	cstr[rootPath.size()] = 0;

	errno = 0;
	int ret = nftw(cstr, rm, 20, FTW_DEPTH);

/*	char * const arr[] {cstr, NULL};
  	FTS *fts = fts_open(arr, FTS_LOGICAL | FTS_NOCHDIR, NULL);


	if (fts != NULL)
	{
		FTSENT *item = NULL;
		while ((item = fts_read(fts)) != NULL)
		{
			if ((item->fts_info == FTS_DP) || (item->fts_info == FTS_F))
			{
				remove(item->fts_path);
			}
		}
	}

	fts_close(fts);
    */
}


// Not yet implemented section:
int azs_access(const char *path, int mask)
{
	return 0;  // permit all access
}

int azs_readlink(const char *path, char *buf, size_t size)
{
	return 1; // ignore for mow
}

int azs_fsync(const char *path, int isdatasync, struct fuse_file_info *fi)
{
	return 0; // Skip for now
}

int azs_chown(const char *path, uid_t uid, gid_t gid)
{
	//TODO: Implement
	return 0;
}

int azs_chmod(const char *path, mode_t mode)
{
	//TODO: Implement
	return 0;

}

//#ifdef HAVE_UTIMENSAT
int azs_utimens(const char *path, const struct timespec ts[2])
{
	//TODO: Implement
	return 0;
}
//  #endif



int azs_truncate(const char *path, off_t off)
{
	//TODO: Implement
	return 0;
}

int azs_rename(const char *src, const char *dst)
{
	//TODO: implement
	return 0;
}


int azs_setxattr(const char *path, const char *name, const char *value, size_t size, int flags)
{
	return 0;
}
int azs_getxattr(const char *path, const char *name, char *value, size_t size)
{
	return 0;
}
int azs_listxattr(const char *path, char *list, size_t size)
{
	return 0;
}
int azs_removexattr(const char *path, const char *name)
{
	return 0;
}
