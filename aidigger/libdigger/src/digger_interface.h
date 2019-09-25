#ifndef DIGGER_INTERFACES_H
#define DIGGER_INTERFACES_H
#include <pthread.h>
#include "join_pics.h"
//init the network used for yolov3, return a network_ptr
void* init_yolov3_data(const char* weight_file, const char* cfg, const char* coco_names, const char** picNames);

//create a thread given rand seed, picNames, network_ptr (yolo v3 network weights)
pthread_t creat_thread(int rand_seed, const char** picNames, void* network_ptr, int thread_count);

void cancel_thread(pthread_t thread);
int get_result(pthread_t thread, unsigned char* result);
void test();
#endif

