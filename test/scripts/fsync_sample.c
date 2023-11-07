#include <stdio.h>
#include <stdlib.h>
#include <fcntl.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <string.h>
#include <unistd.h>

int main(int argc, char *argv[]) {
   char *data = "Testing fsync method";
   int file, r;

   file = creat("/usr/blob_mnt/fsync.txt", S_IWUSR | S_IRUSR);
   if (file < -1) {
      perror("creat()");
      exit(1); 
   }

   r = write(file, data, strlen(data));
   if(r < -1) {
      perror("write()");
      exit(1); 
   }
   
   fsync(file);
   close(file);
   
   return 0;
}
