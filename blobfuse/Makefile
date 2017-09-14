blobfuse: blobfuse.cpp directoryapis.cpp fileapis.cpp utilities.cpp blobfuse.h
	g++ blobfuse.cpp directoryapis.cpp fileapis.cpp utilities.cpp `pkg-config fuse --cflags --libs` -std=c++11 -I ../azure-storage-cpp-light/src/include -lcurl -L ../azure-storage-cpp-light/src/build -lazure-storage -ggdb -o blobfuse

clean: blobfuse
	rm blobfuse
