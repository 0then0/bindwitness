#ifndef SYMBOL
#define SYMBOL shared_value
#endif
extern int SYMBOL(int);
int run(int x) { return SYMBOL(x); }
