#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <unistd.h>

/* Change cwd before loading a relative provider; reference stays absolute. */
int main(int argc, char **argv) {
 if (argc != 3 || chdir(argv[1])) return 20;
 if (!dlopen("./librelative.so", RTLD_LAZY | RTLD_GLOBAL)) return 21;
 void *consumer = dlopen(argv[2], RTLD_LAZY);
 if (!consumer) return 22;
 int (*run)(int) = (int (*)(int))dlsym(consumer, "run");
 if (!run) return 23;
 printf("result=%d\n", run(5));
 return 0;
}
