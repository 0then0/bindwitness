#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/wait.h>
int main(int argc, char **argv) {
 if (argc != 5) return 20;

 const char *expected_argv0 = getenv("BW_EXPECT_ARGV0");
 if (expected_argv0 && strcmp(argv[0], expected_argv0)) return 33;
 /* Absolute loader names keep the fixture's file identities unambiguous. */
 for (int i = 1; i <= 3; ++i) {
  char *path = realpath(argv[i], NULL);
  if (!path) return 34;
  argv[i] = path;
 }
 void *first = dlopen(argv[1], RTLD_LAZY | RTLD_GLOBAL);
 void *second = dlopen(argv[2], RTLD_LAZY | RTLD_GLOBAL);
 if (!first || !second) { fprintf(stderr, "%s\n", dlerror()); return 21; }
 void *consumer;
 if (!strcmp(argv[4], "namespace")) consumer = dlmopen(LM_ID_NEWLM, argv[3], RTLD_LAZY);
 else consumer = dlopen(argv[3], RTLD_LAZY);
 if (!consumer) { fprintf(stderr, "%s\n", dlerror()); return 22; }
 int (*run)(int) = (int (*)(int)) dlsym(consumer, "run");
 if (!run) return 23;
 int result = run(5); printf("result=%d\n", result);
 if (!strcmp(argv[4], "reload")) {
  dlclose(consumer); consumer = dlopen(argv[3], RTLD_LAZY);
  if (!consumer) return 24;
  run = (int (*)(int)) dlsym(consumer, "run"); if (!run || run(5) != 12) return 25;
 }
 if (!strcmp(argv[4], "noise")) {
  char buf[4096]; memset(buf, 'x', sizeof(buf));
  for (int i = 0; i < 1024; ++i) { if (fwrite(buf, 1, sizeof(buf), stdout) != sizeof(buf)) return 26; if (fwrite(buf, 1, sizeof(buf), stderr) != sizeof(buf)) return 27; }
 }
 if (!strcmp(argv[4], "sleep")) sleep(30);
 if (!strcmp(argv[4], "signal")) {
  const char *path = getenv("BW_PID_FILE");
  if (!path) return 35;
  FILE *f = fopen(path, "w");
  if (!f) return 36;
  if (fprintf(f, "%ld\n", (long)getpid()) < 0 || fclose(f)) return 37;
  sleep(30);
 }
 if (!strcmp(argv[4], "mutate")) {
  FILE *f = fopen(argv[1], "a");
  if (!f) return 30;
  if (fputs("artifact mutation", f) < 0) { fclose(f); return 31; }
  if (fclose(f)) return 32;
 }
 if (!strcmp(argv[4], "child")) {
  pid_t p = fork(); if (p == 0) { execl("/bin/echo", "echo", "child", (char *)0); _exit(29); }
  if (p < 0) return 28;
  waitpid(p, 0, 0);
 }
 if (!strcmp(argv[4], "malformed")) fprintf(stderr, "%ld:\tbinding file broken normal symbol `shared_value'\n", (long)getpid());
 if (!strcmp(argv[4], "nonzero")) return 7;
 return result != 12;
}
