#ifndef JOIN_PIC_DETECT_H
#define JOIN_PIC_DETECT_H
char* CFG ;
char* WEIGHTS_FILE;
char* COCONAME;

void print_bytes(unsigned char* bytes, int len, char* name);
int join_pic_detect(int rand_seed, const char** picNames,unsigned char* result, void* network_ptr, unsigned long thread);
void print_hello_join_pic_detect();
void* initNetwork(char *cfgfile,char *weightfile);
void enter_to_continue();
#endif