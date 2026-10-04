#ifndef SYMBOL
#define SYMBOL shared_value
#endif
#ifndef OFFSET
#define OFFSET 7
#endif
int SYMBOL(int x) { return x + OFFSET; }
/* Intentionally never called by the ordinary smoke workload. */
int dormant(int x) { return x + 7; }
