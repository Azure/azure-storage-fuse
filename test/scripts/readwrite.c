#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>

// To build this just run : gcc readwrite.c -o readwrite
// To execute this run : ./readwrite

int main()
{
    FILE *wfp = fopen("./mnt/blobfuse_mnt/f12.txt", "w");
    FILE *rfp = fopen("./mnt/blobfuse_mnt/f12.txt", "r");
    FILE *xfp = fopen("./mnt/blobfuse_mnt/f12.txt", "r");
    char data[200];
    int r = 0;

    printf("Writing to file\n");
    fprintf(wfp, "This is read from file 1\n");
    fprintf(wfp, "This is read from file 2\n");

    printf("Reading before flush\n");
    r = fread(data, 1, 200, rfp);
    printf("Bytes read before flush (not exp) : %d\n", r);

    printf("Flushing\n");
    fflush(wfp);

    printf("Write after flush\n");
    fprintf(wfp, "This is read from file 3\n");

    fseek(rfp, 0, SEEK_SET);
    printf("Read after flush\n");
    r = fread(data, 1, 200, rfp);
    printf("Bytes read after flush (2lines exp) : %d\n", r);


    printf("Closing\n");
    fclose(wfp);

    printf("Read after close\n");
    r = fread(data, 1, 200, xfp);
    printf("Bytes read after flush (3lines exp) : %d\n", r);


    printf("Closing read handles\n");
    fclose(rfp);
    fclose(xfp);
}