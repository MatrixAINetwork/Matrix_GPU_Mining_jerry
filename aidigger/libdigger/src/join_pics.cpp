#define cimg_use_jpeg
#include "join_pics.h"
#include <stdio.h>
#include <tuple>
#include <vector>
#include <stdlib.h>
#include <dirent.h>
#include <string.h>
#include <iostream>
#include <chrono>
#include "CImg.h"
#include "sclog4c/sclog4c.h"

using namespace cimg_library;
CImg<unsigned char> imgs[16];

std::vector<int> gen_rand_list(int list_size, int max_value){
    
    std::vector<int> rand_list;
    for(int j = 0; j < list_size; j++){
        int r = rand()%max_value;
        rand_list.push_back(r);        
    }    
    return rand_list;
}

std::vector<std::tuple<int,int,int,int>> cal_anker_points(int img_size_x, int img_size_y, int divide_x, int divide_y, bool rand_axis_is_x = false){    
    std::vector<std::tuple<int,int,int,int>> anker_points;    
    if(!rand_axis_is_x){
        int step_x = img_size_x/divide_x;        
        int anker_x = 0;
        int tile_size_x = step_x;
        for(int i = 0; i < divide_x; i++){
            std::vector<int> rand_list = gen_rand_list(divide_y,10);
            int sum = 0;
            for(int r : rand_list){    
                r = r + 1; //plus 1 for avoiding divide zero          
                //logm(SL4C_DEBUG, "r is %d\n", r);
                sum += r; 
            }
            std::vector<int> anker_y_list;
            int sum_so_far = 0;
            //logm(SL4C_DEBUG, "sum is %d", sum);
            for(int r: rand_list){                
                r = r + 1;
                int anker_y = int(sum_so_far * img_size_y/sum);
                int tile_size_y = r * img_size_y/sum;
                anker_points.push_back(std::make_tuple(anker_x,anker_y,tile_size_x,tile_size_y));
                sum_so_far += r;                
            }
            anker_x += step_x;            
        }
    }
    else{
        int step_y = img_size_y/divide_y;
        int anker_y = 0;
        int tile_size_y = step_y;
        for(int i = 0; i < divide_y; i++){
            std::vector<int> rand_list = gen_rand_list(divide_x,10);
            int sum = 0;
            for(int r : rand_list){
                r = r + 1;
                sum += r; //plus 1 for avoiding divide zero;
            }
            std::vector<int> anker_x_list;
            int sum_so_far = 0;
            for(int r: rand_list){              
                r = r + 1;  
                int anker_x = int(sum_so_far * img_size_x/sum);
                int tile_size_x = r * img_size_x/sum;
                anker_points.push_back(std::make_tuple(anker_x,anker_y, tile_size_x,tile_size_y));
                sum_so_far += r;                                
            }
            anker_y += step_y;
        }           
    }
    return anker_points;
}

void fill_image_with_image(CImg<unsigned char>& dest,CImg<unsigned char>& source, int anker_x = 0, int anker_y =  0){
    
    const double 
        bb_x_lu = anker_x,
        bb_y_lu = anker_y,
        bb_x_rd = anker_x + source.width(),
        bb_y_rd = anker_y + source.height();
    int
        dx = source.width(),
        dy = source.height(),
        dz = std::max(dest.depth(),source.depth()),
        dv = std::max(dest.spectrum(),source.spectrum());
                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    
    source.resize(dx,dy,dz,dv);

    //static_assert(bb_x_rd < dest.width() && bb_y_rd < dest.height());
        
    cimg_forXYZC(dest,x,y,z,k) {
        if (x >= bb_x_lu && x <= bb_x_rd && y >= bb_y_lu && y <= bb_y_rd){
            dest(x,y,z,k) = source(x - bb_x_lu, y - bb_y_lu,z,k);
        }
    }
}

bool has_suffix(const std::string &str, const std::string &suffix)
{
    return str.size() >= suffix.size() &&
           str.compare(str.size() - suffix.size(), suffix.size(), suffix) == 0;
}

std::vector<std::string> choose_image(const char* path, int img_number){
    std::vector<std::string> chose_img_names;
    DIR *dir = opendir(path);
    char buff[256];
    struct dirent *dp;

    int pic_count = 0;
    std::vector<std::string> img_names;
    while ((dp = readdir(dir)) != NULL)
    {
        char* absFilename = buff;
        strcpy(absFilename,path);               
        strcat(absFilename,dp->d_name); 
        if(!has_suffix(absFilename,".jpg"))
            continue;
        
        img_names.push_back(absFilename);
        pic_count++;        
    }
    printf("pic count is %d\n", pic_count);
    std::vector<int> rand_list = gen_rand_list(img_number, pic_count);
    for(auto r:rand_list){
        chose_img_names.push_back(img_names.at(r));
    }
    return chose_img_names;
}

CImg<unsigned char> get_rand_crop(const CImg<unsigned char>& img, int crop_size_x, int crop_size_y){
    
    int rand_anker_x = rand()%(img.width()-crop_size_x);
    int rand_anker_y = rand()%(img.height()-crop_size_y);
    
    CImg<unsigned char> tmp_img(img);

    while((rand_anker_x+crop_size_x > tmp_img.width())||(rand_anker_y + crop_size_y > tmp_img.height()))
        tmp_img = tmp_img.get_resize_doubleXY();

    return tmp_img.get_crop(rand_anker_x,rand_anker_y,rand_anker_x+crop_size_x, rand_anker_y + crop_size_y);
}

CImg<unsigned char> rand_join_pics(int dest_size_x, int dest_size_y, int divide_x, int divide_y,const char* pic_paths){
    CImg<unsigned char> dest(dest_size_x,dest_size_y,3,3);
    std::vector<std::tuple<int,int,int,int>> anker_points = cal_anker_points(dest_size_x,dest_size_y,divide_x,divide_y);
    std::vector<std::string> img_names = choose_image(pic_paths, anker_points.size());
    int pic_id = 0;
    for(auto anker_point:anker_points){
        std::string pic_name = img_names.at(pic_id);
        CImg<unsigned char> source; 
        source.load_jpeg(pic_name.c_str());
        //source.display("emmm");
        int 
            anker_x = std::get<0>(anker_point),
            anker_y = std::get<1>(anker_point),
            crop_x = std::get<2>(anker_point),
            crop_y = std::get<3>(anker_point);

        //std::cout<<"anker_x "<<anker_x<<" anker_y "<<anker_y<<" size_x "<<crop_x<<" crop_y "<<crop_y<<std::endl;

        CImg<unsigned char> crop = get_rand_crop(source,crop_x,crop_y);
        //crop.display("crop");
        fill_image_with_image(dest,crop,anker_x,anker_y);
        pic_id++;
    }

    return dest;
}

#ifdef __cplusplus
extern "C"
{
#endif
#define PICS_PATH ""

int load_16_imgs(const char** picNames){
    try{
        printf("loading pics\n");
        char buff[256];  
        const char* path = PICS_PATH;
        for (int i = 0; i < 16; i++){
                char* absFilename = buff;
                strcpy(absFilename,path);  
                const char* picName_chars = picNames[i];
                strcat(absFilename,picName_chars); 
                CImg<unsigned char>* source = &imgs[i];
                source->load_jpeg(absFilename);
        }
        printf("finished loading pics\n");
        return 1;
    }    
    catch(...){
        return 0;
    }
}

int join_16_pics(int rand_seed, const char** picNames,int join_pic_sizex, int join_pic_sizey, const char* join_pic_name){
    srand(rand_seed);
    try{
        logm(SL4C_DEBUG,"begin join join 16 pic for %s\n", join_pic_name);
        auto begin = std::chrono::high_resolution_clock::now();
        CImg<unsigned char> dest(join_pic_sizex,join_pic_sizey,3,3);
        logm(SL4C_DEBUG,"cal ankers pic for %s\n", join_pic_name);
        std::vector<std::tuple<int,int,int,int>> anker_points = cal_anker_points(join_pic_sizex,join_pic_sizey,4,4);
        int pic_id = 0;
        char buff[256];    
        const char* path = PICS_PATH;
        logm(SL4C_DEBUG,"fill image with images for %s\n", join_pic_name);
        for(auto anker_point:anker_points){

            CImg<unsigned char> *source = &imgs[rand()%16];
    
            int 
                anker_x = std::get<0>(anker_point),
                anker_y = std::get<1>(anker_point),
                crop_x = std::get<2>(anker_point),
                crop_y = std::get<3>(anker_point);

            //std::cout<<"anker_x "<<anker_x<<" anker_y "<<anker_y<<" size_x "<<crop_x<<" crop_y "<<crop_y<<std::endl;

            CImg<unsigned char> crop = get_rand_crop(*source,crop_x,crop_y);
            //crop.display("crop");
            fill_image_with_image(dest,crop,anker_x,anker_y);
            pic_id++;
        }
        logm(SL4C_DEBUG,"saving pic for %s\n", join_pic_name);
        dest.save(join_pic_name);
        //dest.display("result");
        // auto end = std::chrono::high_resolution_clock::now();
        // auto dur = end - begin;
        // auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(dur).count();    
        // std::cout << "finished joining pic in "<<ms <<" ms"<< std::endl; 
        logm(SL4C_DEBUG, "returning join pic function for %s\n", join_pic_name);
        return 1;
    }
    catch(...){
        return 0;
    }
}

int join_pics(int rand_seed, int width,int height,int divide_x,int divide_y,const char* pics_path, const char* join_pic_name){ 

    srand(rand_seed);
    auto begin = std::chrono::high_resolution_clock::now();
    CImg<unsigned char> result = rand_join_pics(width,height,divide_x,divide_y,pics_path);
    auto end = std::chrono::high_resolution_clock::now();
    auto dur = end - begin;
    auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(dur).count();    
    std::cout << "finished in "<<ms <<" ms"<< std::endl; 
    char buff[256];
    result.save(join_pic_name);
    result.display();
    return 1;
}



#ifdef __cplusplus
} // extern "C"
#endif


