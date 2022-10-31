# Blobfuse2 Stream (Preview)

## About

Blobfuse2 Stream is a feature which helps support reading and writing large files that will not fit in the file cache on the local disk. It also provides performance optimization for scenarios where only small portions of a file are accessed since the file does not have to be downloaded in full before reading or writing to it. It supports the following modes

1. **File-Handle based Caching**
    - Separate file handles have separate buffers irrespective of whether or not they point to the same file
    - Ideal for scenarios where multiple handles are reading from different parts of a file
    - Not recommended to be used for multiple writer or single writer multiple reader scenarios
    - If writing through multiple handles, the last handle closed will win and may not persist writes from previously closed handles if their data buffers overlap
    - If writing on one handle, modified data will only be visible by handles opened after the writer handle closes. 

2. **File-Name based Caching**
    - Separate file handles pointing to the same file share buffers
    - Behaves most closely to the file cache in Blobfuse
    - Ideal for scenarios where multiple handles are reading from close by parts of a file and multiple writer or single writer multiple reader

## Enable Stream

To enable stream, first specify stream under the components sequence between libfuse and attr_cache. Note 'stream' and 'file_cache' currently can not co-exist.

```yaml
components:
    - libfuse
    - stream
    - attr_cache
    - azstorage
```

The different configuration options for stream are,
- `block-size-mb: 16`: Integer parameter that specifies the size of each block to be cached in memory (in MB). 
- `max-buffers: 16`: Integer parameter that specifies the total number of buffers to be cached in memory (in MB). 
- `buffer-size-mb: 16`: Integer parameter that specifies the size of each buffer to be cached in memory (in MB). 
- `file-caching: true|false`: Boolean parameter to specify file name based caching. Default is false which specifies file handle based caching.

### Sample Config

After adding the components, add the following section to your blobfuse2 config file. The following example enables Blobfuse2 to use up to 64 * 128 MB of memory to cache data buffers with file handle based caching
```yaml
stream:
  block-size-mb: 64
  max-buffers: 128
  buffer-size-mb: 64
  file-caching: false
```