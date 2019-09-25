#include "digger_interface.h"
#include "join_pic_detect.h"
#include <stdio.h>
#include <unistd.h>
#include "sclog4c/sclog4c.h"
#include "join_pics.h"
#include <stdlib.h>

#ifdef WIN32
#include <windows.h>
#elif _POSIX_C_SOURCE >= 199309L
#include <time.h>   // for nanosleep
#else
#include <unistd.h> // for usleep
#endif


void sleep_ms(int milliseconds) // cross-platform sleep function
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


int numberInArray(int a[], int arraysize,int number){
    for (int i =0; i <arraysize; i++){
        if(a[i] == number)
            return 1;            
    }
    return 0;
}



int main(int argc, char **argv){  
    // test();
    // return 0;
    
    srand(0);
    if(argc < 2){
        fprintf("usage: %s <function>, rand_seed\n", argv[0]);
        return 0;
    }
    //sclog4c_level = SL4C_ALL;
    int rand_seed = strtol(argv[1], NULL, 10);   
    // logm(SL4C_DEBUG,"Program name: %s, %d", argv[0], rand_seed);
    // logm(SL4C_DEBUG,"logging sth\n");
    // return 0;

    const char* picNames[] = {"./16_testPics/00d66ed55093c3bf.jpg",  "./16_testPics/0222359686b52503.jpg",  "./16_testPics/03b34394c4fae1d2.jpg",  "./16_testPics/0574623c2473a463.jpg",  "./16_testPics/076c438efda49fac.jpg"  ,"./16_testPics/0973221d1bc979c1.jpg",  "./16_testPics/0b96750f7bfbef43.jpg",  "./16_testPics/0dc5f1cf71842cbe.jpg",
"./16_testPics/00d67ab9e6db2059.jpg",  "./16_testPics/0222397d2ce9241e.jpg",  "./16_testPics/03b351e2faa608fe.jpg",  "./16_testPics/057463e74cc756bd.jpg",  "./16_testPics/076c44dc65599558.jpg",  "./16_testPics/097335b71ef0ebca.jpg",  "./16_testPics/0b967884421ea018.jpg",  "./16_testPics/0dc6006b96ae1213.jpg"
};
    void* network_ptr = init_yolov3_data("yolov3.weights","yolov3.cfg", "coco.names", picNames);
  
    logm(SL4C_DEBUG,"finished loading network and loading 16 images. Now lets detect\n");
    // for(int i = 0; i < 10; i++){
    //     join_16_pics(rand_seed,picNames,400,400, "join16test");
    //     enter_to_continue();
    // }
    //return 0;

    #define total_threads  1000
    unsigned long threads[total_threads];
    int got_results[total_threads];
    
    // unsigned char** results = malloc(5*32);
    // for(int i = 0; i < threads_num; i++){
    //     unsigned long thread = creat_thread(rand_seed+i, picNames,network_ptr);
    //     threads[i] = thread;
    //     sleep_ms(500);
    // }
    //sleep(2);
    // unsigned long thread1 = creat_thread(rand_seed, picNames,network_ptr);
    // printf("im herer\n");

    // unsigned char* result = malloc(32);
    // //unsigned char* result1 = malloc(32);


    int tic = 0;
    
    int thread_count = 0;
    int finished_threads[total_threads];
    int rand_nums[total_threads];
    int finished_count = 0;

    for(int i = 0; i < total_threads;i++)
        finished_threads[i] = -1;
    FILE *fptr;
    fptr = fopen("thread_results.txt", "w");
    while(1){
            

        logm(SL4C_DEBUG,"creating a thread %d\n", thread_count);
        int rand_num = rand_seed + thread_count;
        unsigned long thread = creat_thread(rand_num, picNames,network_ptr,thread_count);
        if (thread > 0){
            //enter_to_continue();
            logm(SL4C_DEBUG,"succeed created a thread %lu, thread count %d, rand num %d\n", thread, thread_count, rand_num);
            // fprintf(fptr,"thread %lu,thread count %d\n", thread,thread_count);
            
            threads[thread_count] = thread;
            rand_nums[thread_count] = rand_num;
            
            thread_count++;            

        }
        

        //enter_to_continue();

        logm(SL4C_DEBUG,"thead_coutn is %d\n", thread_count);
        while(1){
            for(int i = 0; i < thread_count; i++){              

                if(numberInArray(finished_threads,total_threads,i))
                    continue;
                logm(SL4C_DEBUG,"im herer with total thread = %d\n", thread_count);
                // char buffer[26];
                // time_t timer;
                // time(&timer);
                // struct tm* tm_info;
                // tm_info = localtime(&timer);
                // strftime(buffer, 26, "%Y-%m-%d %H:%M:%S", tm_info);
                // puts(buffer);
                unsigned char* result = malloc(32);
                logm(SL4C_DEBUG,"getting %lu thread result, tread count %d\n", threads[i], i);                
                int succeed = get_result(threads[i], result);
                if(succeed){
                    printf("succeed!");
                    logm(SL4C_DEBUG,"thread count %d, rand_num %d result \n", i, rand_nums[i]);  
                    print_bytes(result, 32, "result in main");
                    fprintf(fptr,"thread %lu,thread count %d, rand %d, ", threads[i],i,rand_nums[i]);
                    for(int i = 0; i < 32; i++){
                        fprintf(fptr,"0x%x, ",*(result+i));
                    }
                    fprintf(fptr,"\n");
                    succeed = 0;
                    finished_threads[finished_count] = i;
                    finished_count++;
                    //enter_to_continue();
                    
                    //break;
                }
                free(result);
                
            }             
            sleep_ms(200);            
            logm(SL4C_DEBUG,"tick is %d\n", tic);  
            tic++;
            //enter_to_continue();

            if(finished_count == thread_count)
                break;
        }

        if(thread_count == total_threads)
            break;
    }
    fclose(fptr);
    //logm(SL4C_DEBUG,"cant wait any more, abort!\n");
    //cancel_thread(thread);
    //wait_for_thread(thread);
    
    return 0;    
}
