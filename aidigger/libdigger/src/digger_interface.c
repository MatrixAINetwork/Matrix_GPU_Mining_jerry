#include "digger_interface.h"
#include "join_pic_detect.h"
#include <pthread.h>
#include <unistd.h>
#include <stdio.h>
#include "sclog4c/sclog4c.h"
#include "md5.h"
#include <time.h>
#include <string.h>

#ifdef WIN32
#include <windows.h>
#elif _POSIX_C_SOURCE >= 199309L
#include <time.h>   // for nanosleep
#else
#include <unistd.h> // for usleep
#endif


void sleep_ms_local(int milliseconds) // cross-platform sleep function
{
#ifdef WIN32
    Sleep(milliseconds);
#elif _POSIX_C_SOURCE >= 199309L
    struct timespec ts;
    ts.tv_sec = milliseconds / 1000;
    ts.tv_nsec = (milliseconds % 1000) * 1000000;
    nanosleep(&ts, NULL);
#else
    usleep(milliseconds * 1000);
#endif
}


#define MAX_THREAD_NUM 10

#define WEIGHTS_FILE_MD5 {0xc8, 0x4e, 0x5b, 0x99, 0xd0, 0xe5, 0x2c, 0xd4, 0x66, 0xae, 0x71, 0x0c, 0xad, 0xf6, 0xd8, 0x4c}
#define CFG_MD5 {0x9b, 0x7d, 0x21, 0xd6, 0xbb, 0xf6, 0x3a, 0x7c, 0xa9, 0xb6, 0x38, 0x4d, 0x6c, 0xf6, 0x4a, 0x2e}
#define COCO_NAME_MD5 {0x8f, 0xc5, 0x5, 0x61, 0x36, 0x1f, 0x8b, 0xcf, 0x96, 0xb0, 0x17, 0x70, 0x86, 0xe7, 0x61, 0x6c}

int cancel_handler_called_time = 0;
extern char* WEIGHTS_FILE;

struct thread_args {
    long rand_seed;
    unsigned char* result;
    const char** picNames;
    void* network_ptr;
    int thread_count
};


struct thread_stats{
    pthread_t thread;
    int started;
    int finished;
    int read;
    int canceled;
    int thread_count;
    pthread_cond_t cond;     
};

struct thread_stats THREADS_STATS[MAX_THREAD_NUM];

// // Declaration of thread condition variable 
// pthread_cond_t cond1 = PTHREAD_COND_INITIALIZER; 
  
// declaring mutex 
pthread_mutex_t lock = PTHREAD_MUTEX_INITIALIZER; 

void* init_yolov3_data(const char* weight_file, const char* cfg, const char* coco_names, const char** picNames){
    logm(SL4C_DEBUG, "loading pics");
    load_16_imgs(picNames);
    
    const unsigned char weight_file_md5[] = WEIGHTS_FILE_MD5;
    const unsigned char cfg_md5[] = CFG_MD5;
    const unsigned char coco_name_md5[] = COCO_NAME_MD5;
    int validate = 0;
    WEIGHTS_FILE = malloc(256);    
    COCONAME = malloc(256);
    CFG = malloc(256);


    strcpy(WEIGHTS_FILE,weight_file);
    strcpy(CFG,cfg);
    strcpy(COCONAME,coco_names);


    validate = validate_md5(WEIGHTS_FILE,weight_file_md5);
    if(validate != 1){
        logm(SL4C_DEBUG, "weight file currupted\n");
        return NULL;
    }
    else{
        logm(SL4C_DEBUG, "weight file is correct!\n");
    } 
   
    validate = validate_md5(CFG,cfg_md5);
    if(validate != 1){
        logm(SL4C_DEBUG, "cfg file currupted\n");
        return NULL;
    }
    else{
        logm(SL4C_DEBUG, "cfg file is correct!\n");
    } 
    validate = validate_md5(COCONAME,coco_name_md5);
    if(validate != 1){
        logm(SL4C_DEBUG, "coco name file currupted\n");
        return NULL;
    }
    else{
        logm(SL4C_DEBUG, "coco name file is correct!\n");
    }

    void* net = initNetwork(CFG, WEIGHTS_FILE);    
    return net;
}


static void cleanup_handler(void *arg){
    cancel_handler_called_time++;
    printf("Called clean-up handler the %d time\n",cancel_handler_called_time);    
}

void enter_to_continue(){
    logm(SL4C_DEBUG, "Press enter to continue\n");
    char enter = 0;
    while (enter != '\r' && enter != '\n') { enter = getchar(); }
    logm(SL4C_DEBUG, "Thank you for pressing enter\n");
}

struct thread_stats * find_thread_stats(pthread_t thread){
    logm(SL4C_DEBUG,"have threads\n");
    for(int i = 0; i < MAX_THREAD_NUM; i++){
        struct thread_stats* sts = &THREADS_STATS[i];
        logm(SL4C_DEBUG,"thread %lu, thread_count %d\n", sts->thread, sts->thread_count);
        if(sts->thread == thread)
            return sts;
    }
    return NULL;   
}

void* thread_func(void* _args){
    pthread_cleanup_push(cleanup_handler, NULL);
    /* set thread cancel type to asynchronous to make thread quit as soon as possible */
    int rc = pthread_setcanceltype(PTHREAD_CANCEL_ASYNCHRONOUS, NULL);
    logm(SL4C_DEBUG, "pthread_setcanceltype() %lu\n", rc);

    pthread_t  self;
    self = pthread_self();
    logm(SL4C_DEBUG, "creating thread %lu\n", self);

    struct thread_args *args = (struct thread_args *) _args;
    /* set thread status */ 
    struct thread_stats* sts = NULL;
    for(int i = 0; i < MAX_THREAD_NUM; i++){
        sts = &THREADS_STATS[i];
        if((!sts->started) || (sts->read) || (sts->canceled)){
            logm(SL4C_DEBUG, "set thread %lu stats \n", self);
            sts->thread = self;
            sts->started = 1;
            sts->finished = 0;
            sts->read = 0;
            sts->canceled = 0;
            sts->thread_count = args->thread_count;
            sts->cond = (pthread_cond_t)PTHREAD_COND_INITIALIZER; 
            break;            
        }
        sts = NULL;
    }

    if(sts == NULL){
        logm(SL4C_DEBUG, "cannot creat new thread %d, thread pool full\n", args->thread_count);
        return;
    }
    pthread_mutex_lock(&lock); 
    long rand_seed =  args->rand_seed;            
    unsigned char* result = args->result;     
    void* network_ptr = args->network_ptr;
    const char* picNames = args->picNames;
    free(_args);
    int succeed  = join_pic_detect(rand_seed, picNames, result, network_ptr, self); 
    logm(SL4C_DEBUG, "thread %lu is finished with succed %d\n", self, succeed);
    sts->finished = 1;     
    pthread_cond_wait(&(sts->cond), &lock);     
    pthread_mutex_unlock(&lock);
    if(succeed)
        pthread_exit(result);
    else
        pthread_exit(NULL);
    pthread_cleanup_pop(0);
}

unsigned char* wait_for_thread(pthread_t thread){
    unsigned char* t_result;    
    struct thread_stats* sts = find_thread_stats(thread);
    if (sts == NULL){
        logm(SL4C_DEBUG,"getting thread %lu result, but %lu thread does not exits\n", thread);
        return NULL;
    }
    else{        
        logm(SL4C_DEBUG, "getting thread %d reasult\n", sts->thread_count);
        pthread_join(thread, &t_result);    
        logm(SL4C_DEBUG, "got %d thread reasult\n", sts->thread_count);
    }
    return t_result;
}

pthread_t creat_thread(long rand_seed, const char** picNames, void* network_ptr, int thread_count){
    
    struct thread_args *args = malloc (sizeof (struct thread_args));
    args->rand_seed = rand_seed;
    args->result = malloc(32);
    args->picNames = picNames;
    args->network_ptr = network_ptr;
    args->thread_count = thread_count;

    pthread_t thread;
    pthread_create(&thread, NULL, thread_func, args);
    sleep_ms_local(100);
    struct thread_stats* sts = find_thread_stats(thread);
    if (sts == NULL){
        logm(SL4C_DEBUG,"create thread failed %lu\n", thread);
        return 0;
    }
    else{
        logm(SL4C_DEBUG,"successfully create thread %lu\n", thread);
        return thread;    
    }
}

void cancel_thread(pthread_t thread){
    
    struct thread_stats* sts = find_thread_stats(thread);
    if(sts == NULL){
        logm(SL4C_DEBUG, "thread %lu does not exist\n", thread);
        return;
    }    
    pthread_cancel(thread);
    sts->canceled = 1;
    logm(SL4C_DEBUG, "cancelled a thread %d\n", sts->thread_count);

}



int get_result(pthread_t thread, unsigned char* result){
    // for(int i = 0; i < MAX_THREAD_NUM; i++){
    //     struct thread_stats sts = THREADS_STATS[i];
    //     logm(SL4C_DEBUG, "thread %lu started %lu finished %lu", sts.thread, sts.started, sts.finished);
    // }    
    logm(SL4C_DEBUG,"calling get_result %lu\n", thread);
    char buffer[26];
    time_t timer;
    time(&timer);
    struct tm* tm_info;
    tm_info = localtime(&timer);
    strftime(buffer, 26, "%Y-%m-%d %H:%M:%S", tm_info);
    
    logm(SL4C_DEBUG, "time is %s\n", buffer);

    struct thread_stats* sts = find_thread_stats(thread);
    if(sts == NULL){
        logm(SL4C_DEBUG, "thread %lu does not exist\n", thread);
        return 0;
    }
    else if (!sts->started){
        logm(SL4C_DEBUG, "thread %d not started yet\n", sts->thread_count);
        return 0;        
    }

    else if(!sts->finished){
        logm(SL4C_DEBUG, "thread %d not finished yet\n", sts->thread_count);
        return 0;
    }
    else if(sts->read){
        logm(SL4C_DEBUG, "thread %d has been read\n", sts->thread_count);
        return 0;
    }
    else if (sts->canceled){
        logm(SL4C_DEBUG, "thread %d canceled\n", sts->thread_count);
        return 0;
    }
    else{
        logm(SL4C_DEBUG, "Signaling thread %lu to wake\n", sts->thread_count); 
        pthread_cond_signal(&(sts->cond));
        unsigned char* tmp_result = wait_for_thread(thread);
        memcpy(result, tmp_result, 32);
        sts->read = 1;
        free(tmp_result);
        return 1;
    }
}


void test(){
    for(int i = 0; i < MAX_THREAD_NUM;i++){
        struct thread_stats* sts = &THREADS_STATS[i];
        sts->thread = i;
        sts->started = 1;
        sts->finished = 0;
    }
    for(int i = 0; i < MAX_THREAD_NUM; i++){
        struct thread_stats sts = THREADS_STATS[i];
        logm(SL4C_DEBUG, "thread %lu started %lu finished %lu\n", sts.thread, sts.started, sts.finished);
    }
}