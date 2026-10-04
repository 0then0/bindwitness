#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>

/* LD_BIND_NOT + LD_DYNAMIC_WEAK allow a second binding after a strong export loads. */
int main(int argc, char **argv) {
 if (argc != 4 || !dlopen(argv[1], RTLD_LAZY | RTLD_GLOBAL)) return 20;
 void *consumer = dlopen(argv[3], RTLD_LAZY);
 if (!consumer) return 21;
 int (*run)(int) = (int (*)(int))dlsym(consumer, "run");
 if (!run) return 22;
 printf("result=%d\n", run(5));
 if (!dlopen(argv[2], RTLD_LAZY | RTLD_GLOBAL)) return 23;
 printf("result=%d\n", run(5));
 return 0;
}
