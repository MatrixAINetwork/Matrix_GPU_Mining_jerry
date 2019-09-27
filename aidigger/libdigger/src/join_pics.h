#ifndef JOIN_PIC_H
#define JOIn_PIC_H


#ifdef __cplusplus
#define EXTERNC extern "C"
#else
#define EXTERNC
#endif

EXTERNC int load_16_imgs(const char** picNames);

EXTERNC int join_pics(long rand_seed, int width,int height,int divide_x,int divide_y,const char* pics_path, const char* join_pic_name);
EXTERNC int join_16_pics(long rand_seed, const char** picNames,  int join_pic_sizex, int join_pic_sizey, const char* join_pic_name);
#endif