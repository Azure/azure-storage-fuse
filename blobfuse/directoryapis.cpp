#include "blobfuse.h"

int azs_mkdir(const char *path, mode_t mode)
{
	if (AZS_PRINT)
	{
		fprintf(stdout, "mkdir called with path = %s\n", path);
	}

	std::string pathstr(path);
	pathstr.insert(pathstr.size(), "/" + directorySignifier);

	std::istringstream emptyDataStream("");

	std::vector<std::pair<std::string, std::string>> metadata;
	errno = 0;
	azure_blob_client_wrapper->upload_block_blob_from_stream(str_options.containerName, pathstr.substr(1), emptyDataStream, metadata);
	if (errno != 0)
	{
		return 0 - map_errno(errno);
	}
	return 0;
}

/**
 * Read the contents of a directory.  For each entry to add, call the filler function with the input buffer,
 * the name of the entry, and additional data about the entry.  TODO: Keep the data (somehow) for latter getattr calls.
 *
 * @param  path   Path to the directory to read.
 * @param  buf    Buffer to pass into the filler function.  Not otherwise used in this function.
 * @param  filler Function to call to add directories and files as they are discovered.
 * @param  offset Not used
 * @param  fi     File info about the directory to be read.
 * @param  flags  Not used.  TODO: Consider prefetching on FUSE_READDIR_PLUS.
 * @return        TODO: error codes.
 */
int azs_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t offset, struct fuse_file_info *fi)
{
	if (AZS_PRINT)
	{
		fprintf(stdout, "azs_readdir called with path = %s\n", path);
	}
	std::string pathStr(path);
	if (pathStr.size() > 1)
	{
		pathStr.push_back('/');
	}

	errno = 0;
	std::vector<list_blobs_hierarchical_item> listResults = list_all_blobs_hierarchical(str_options.containerName, "/", pathStr.substr(1));
	if (errno != 0)
	{
		return 0 - map_errno(errno);
	}

	filler(buf, ".", NULL, 0);
	filler(buf, "..", NULL, 0);

	int i = 0;
	if (AZS_PRINT)
	{
		fprintf(stdout, "result count = %lu\n", listResults.size());
	}
	for (; i < listResults.size(); i++)
	{
		int fillerResult;
		// We need to parse out just the trailing part of the path name.
		int len = listResults[i].name.size();
		// Note - this code scans through the string(s) more often than necessary.
		if (len > 0)
		{
			char *nameCopy = (char *)malloc(len + 1);
			memcpy(nameCopy, listResults[i].name.c_str(), len);
			nameCopy[len] = 0;

			char *lasts = NULL;
			char *token = strtok_r(nameCopy, "/", &lasts);
			char *prevtoken = NULL;

			while (token)
			{
				prevtoken = token;
				token = strtok_r(NULL, "/", &lasts);
			}

			if (!listResults[i].is_directory)
			{
				if (prevtoken && (strcmp(prevtoken, directorySignifier.c_str()) != 0))
				{
					struct stat stbuf;
					stbuf.st_mode = S_IFREG | 0777; // Regular file (not a directory)
					stbuf.st_nlink = 1;
					stbuf.st_size = listResults[i].content_length;
					fillerResult = filler(buf, prevtoken, &stbuf, 0); // TODO: Add stat information.  Consider FUSE_FILL_DIR_PLUS.
					if (AZS_PRINT)
					{
						fprintf(stdout, "blob result = %s, fillerResult = %d\n", prevtoken, fillerResult);
					}
				}

			}
			else
			{
				if (prevtoken)
				{
					struct stat stbuf;
					stbuf.st_mode = S_IFDIR | 0777;
					stbuf.st_nlink = 2;
					fillerResult = filler(buf, prevtoken, &stbuf, 0);
					if (AZS_PRINT)
					{
						fprintf(stdout, "dir result = %s, fillerResult = %d\n", prevtoken, fillerResult);
					}
				}

			}


			free(nameCopy);
		}

	}
	if (AZS_PRINT)
	{
		fprintf(stdout, "Done with readdir\n");
	}
	return 0;

}

int azs_rmdir(const char *path)
{
	if (AZS_PRINT)
	{
		fprintf(stdout, "azs_rmdir called with path = %s\n", path);
	}

	std::string pathStr(path);
	if (pathStr.size() > 1)
	{
		pathStr.push_back('/');
	}

	errno = 0;
	int dirStatus = is_directory_empty(str_options.containerName, "/", pathStr.substr(1));
	if (errno != 0)
	{
		return 0 - map_errno(errno);
	}
	if (dirStatus == 0)
	{
		return -ENOENT;
	}
	if (dirStatus == 2)
	{
		return -ENOTEMPTY;
	}

	std::string pathString(path);
	const char * mntPath;
	std::string mntPathString = prepend_mnt_path_string(pathString);
	mntPath = mntPathString.c_str();
	if (AZS_PRINT)
	{
		fprintf(stdout, "deleting file %s\n", mntPath);
	}
	remove(mntPath);

	pathStr.append(".directory");
	int ret = azs_unlink(pathStr.c_str());
	if (ret < 0)
	{
		return ret;
	}



	/*	errno = 0;
		std::vector<list_blobs_hierarchical_item> listResults = list_all_blobs_hierarchical(str_options.containerName, "/", pathStr.substr(1));
		if (errno != 0)
		{
			return 0 - map_errno(errno);
		}

		int i = 0;
		if (AZS_PRINT)
		{
			fprintf(stdout, "result count = %d\n", listResults.size());
		}
		for (; i < listResults.size(); i++)
		{
			if (!listResults[i].is_directory)
			{
				std::string path_to_blob(listResults[i].name);
				path_to_blob.insert(0, 1, '/');
				int res = azs_unlink(path_to_blob.c_str());
				if (res < 0)
				{
					return res;
				}

			}
			else
			{
				std::string path_to_blob(listResults[i].name);
				path_to_blob.insert(0, 1, '/');
				int res = azs_rmdir(path_to_blob.c_str());
				if (res < 0)
				{
					return res;
				}
			}
		}

		std::string pathString(path);
		const char * mntPath;
		std::string mntPathString = prepend_mnt_path_string(pathString);
		mntPath = mntPathString.c_str();
		if (AZS_PRINT)
		{
			fprintf(stdout, "deleting file %s\n", mntPath);
		}
		remove(mntPath);
		*/

	return 0;

}
