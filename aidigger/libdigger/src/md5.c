#include <stdio.h>
#include <openssl/md5.h>

int get_file_md5(const char* filename, unsigned char* md5_result)
{
    unsigned char c[MD5_DIGEST_LENGTH];   
    
    int i;
    FILE *inFile = fopen (filename, "rb");
    MD5_CTX mdContext;
    int bytes;
    unsigned char data[1024];

    if (inFile == NULL) {
        printf ("%s can't be opened.\n", filename);
        return 0;
    }

    MD5_Init (&mdContext);
    while ((bytes = fread (data, 1, 1024, inFile)) != 0)
        MD5_Update (&mdContext, data, bytes);
    MD5_Final (c,&mdContext);
    // for(i = 0; i < MD5_DIGEST_LENGTH; i++) printf("%x", c[i]); // print c84e5b99d0e52cd466ae71cadf6d84c     
    // printf (" %s\n", filename);
    memcpy(md5_result, c, MD5_DIGEST_LENGTH);
    fclose (inFile);
    return 1;
}

int validate_md5(const char* filename ,const unsigned char* truth_md5){
    unsigned char md5_result[MD5_DIGEST_LENGTH]; 
    get_file_md5(filename, md5_result);
    // for(int i = 0; i < MD5_DIGEST_LENGTH; i++) printf("0x%x, ", md5_result[i]); // print c84e5b99d0e52cd466ae71cadf6d84c 
    // printf(" md5_result\n");
    // for(int i = 0; i < MD5_DIGEST_LENGTH; i++) printf("0x%02x, ", truth_md5[i]);
    // printf(" truth\n");
    if(memcmp ( md5_result, truth_md5, MD5_DIGEST_LENGTH) == 0)
        return 1;
    else 
        return 0;    
}