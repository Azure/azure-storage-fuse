
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/inotify.h>

int write_and_flush() {
    FILE *fp = fopen("test_write_and_flush.txt", "wb");
    
    if (fp == NULL) {
        printf("Error opening file\n");
        return 1;
    }

    fwrite("Some binary data", sizeof(char), 16, fp);

    fflush(fp);
    fclose(fp);
    return 0;
}

int write_and_noflush() {
    FILE *fp = fopen("test_write_and_noflush.txt", "wb");
    if (fp != NULL) {
        fwrite("Some binary data", sizeof(char), 16, fp);

        // Simulate a crash
        abort();
    }
    return 0;
}

int write_large_data_and_noflush() { 
    FILE *file = fopen("test_write_large_data_and_noflush.txt", "wb");
    if (file != NULL) {
        // Continuously write data to the file
        for (int i = 0; i < 1000000; ++i) {
            fwrite("Some binary data", sizeof(char), 16, file);
            if (i % 100 == 0) {
                printf("Sleeping for 20sec\n");
                sleep(20);
            }
        }
    }

    abort();
}

int main() {
    // Create a file and write some 1mb data into it and then exit the code without flush
    // write_and_flush();
    //write_and_noflush();
    write_large_data_and_noflush();
    return 0;
}