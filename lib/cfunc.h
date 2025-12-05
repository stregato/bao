#ifndef _CFUNC_H
#define _CFUNC_H

typedef struct Data {
    void* ptr;
    size_t len;
} Data;

typedef struct Result{
    void* ptr;
    size_t len;
    long long hnd;
	char* err;
} Result;

typedef void (*Progress)(long long);

#endif