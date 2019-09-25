# gold_digger
This program takes a random number seed and generates a randomized picture according to the random number.

Light version of convolutional neural network Yolo v3 for objects detection is then applied on this randomized picture, yielding a series of detections.

The detections are fed to a SHA256 hash function, yielding a 256 bit string.

This repository supports:

* Linux
* both cuDNN >= 7.1.1
* CUDA >= 8.0

How to compile:
* To compile for CPU just do `make` on Linux with `GPU=0` in the `Makefile` 
* To compile for GPU set flag `GPU=1` in the `Makefile` 
* a libgold_digger.a (cpu version) has been compiled in bin/static/
    
*For GPU:  Required both [CUDA >= 8.0](https://developer.nvidia.com/cuda-toolkit-archive) and [cuDNN >= 7.1.1](https://developer.nvidia.com/rdp/cudnn-archive)

How to start:
* Download [`yolov3.weights`](https://pjreddie.com/media/files/yolov3.weights) to the `bin` directory
* Enter the `bin` directory and run `./gold_digger {random_seed}`

How to use the library:
* To link the library libgold_digger.a into your program, try the following make constructions, (assuming you have a compiled main.o file):
    On CPU:
        gcc -std=c99 ./obj/main.o -L./bin/static/ -lgold_digger -o bin/gold_digger -L./required_libs -lm -pthread -lX11 -lssl -lcrypto -lstdc++ 

    On GPU:(not working rigth now)
        gcc ./obj/main.o -L./bin/static/ -lgold_digger -o bin/gold_digger -L/usr/X11R6/lib -lm -pthread -lX11 -L/usr/local/cuda/lib64 -lcuda -lcudart -lcublas -lcurand -L/usr/local/cudnn/lib64 -lcudnn -lstdc++ 
    Following the Makefile can give you some insight on how the compile works.

* The functions you can use in the library:
    The functions are declared in src/digger_interfaces.h file:
    
        //init the network used for yolov3, return a network_ptr
        void* init_yolov3_data();

        //create a thread given rand seed, picNames, network_ptr (yolo v3 network weights)
        pthread_t creat_thread(int rand_seed, const char** picNames, void* network_ptr);

        void cancel_thread(pthread_t thread);

        //return 0 means getting result failed, return 1 means getting result succeeded 
        int get_result(pthread_t thread, unsigned char* result);
    A brief example of using these functions are in src/main.c

