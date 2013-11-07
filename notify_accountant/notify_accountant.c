#include <arpa/inet.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/time.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

// Higher priority destructors run later. int16_max seems to be the largest
// GCC will accept.
#define DESTRUCTOR_PRIORITY 0xFFFF

const long half_second = 1000000L / 2;

__attribute__((destructor (DESTRUCTOR_PRIORITY)))
void notify_paccountant(void) {

    struct rusage ru;
    if (getrusage(RUSAGE_SELF, &ru) == 0) {
        if (ru.ru_utime.tv_sec < 1 && ru.ru_utime.tv_usec < half_second) {
            return;
        }
    }

    struct sockaddr server;
    struct sockaddr_in *in_server = (struct sockaddr_in*)&server;

    pid_t pid = getpid();
    char pids[32];
    snprintf(pids, sizeof pids, "%ld\n", (unsigned long)pid);
    
    int fd = socket(AF_INET, SOCK_STREAM, 0);
    in_server->sin_family = AF_INET;
    in_server->sin_addr.s_addr = htonl(0x7f000001);
    in_server->sin_port = htons(7117);

    if (connect(fd, &server, sizeof server) == -1) {
        return;
    }
    write(fd, pids, strlen(pids));

    // Wait for the server to signal to us we can go away.
    char buf[1];
    read(fd, buf, 1);

    asm volatile ("int3;");
}
