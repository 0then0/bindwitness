#include <stdio.h>
#include <string.h>
extern int shared_value(int);
extern int dormant(int);
int main(int argc, char **argv) {
 if (argc > 1 && strcmp(argv[1], "idle") == 0) return 0;
 int n = shared_value(5);
 printf("result=%d\n", n);
 if (argc > 1 && strcmp(argv[1], "dormant") == 0) return dormant(5) != 12;
 return n != 12;
}
