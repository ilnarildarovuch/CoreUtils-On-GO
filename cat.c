#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <getopt.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <errno.h>
#include <ctype.h>

#define VERSION "1.1.0"
#define MAX_LINE_LEN (10 * 1024 * 1024) // 10MB

static int number = 0;
static int number_nonblank = 0;
static int squeeze_blank = 0;
static int show_ends = 0;
static int show_nonprinting = 0;
static int show_tabs = 0;
static int show_all = 0;
static int follow_symlinks = 0;

void usage(const char *progname) {
    fprintf(stderr, "Secure cat %s\n", VERSION);
    fprintf(stderr, "Usage: %s [OPTION]... [FILE]...\n", progname);
    fprintf(stderr, "\nOptions:\n");
    fprintf(stderr, "  -n, --number             number all output lines\n");
    fprintf(stderr, "  -b, --number-nonblank    number nonempty output lines, overrides -n\n");
    fprintf(stderr, "  -s, --squeeze-blank      suppress repeated empty output lines\n");
    fprintf(stderr, "  -E, --show-ends          display $ at end of each line\n");
    fprintf(stderr, "  -v, --show-nonprinting   use ^ and M- notation, except for LFD and TAB\n");
    fprintf(stderr, "  -T, --show-tabs          display TAB characters as ^I\n");
    fprintf(stderr, "  -A, --show-all           equivalent to -vET\n");
    fprintf(stderr, "  -e                       equivalent to -vE\n");
    fprintf(stderr, "  -t                       equivalent to -vT\n");
    fprintf(stderr, "  -L, --follow-symlinks    follow symbolic links (default false)\n");
    fprintf(stderr, "      --version            output version information and exit\n");
    fprintf(stderr, "\nExamples:\n");
    fprintf(stderr, "  %s -n file.txt\n", progname);
    fprintf(stderr, "  %s -v binary.data\n", progname);
}

void print_version(const char *progname) {
    printf("%s %s\n", progname, VERSION);
    printf("Secure version with symlink protection and input validation\n");
}

void handle_combined_options() {
    if (show_all) {
        show_nonprinting = 1;
        show_ends = 1;
        show_tabs = 1;
    }
    if (number_nonblank) {
        number = 1;
    }
}

char *process_nonprinting(const char *s) {
    if (!(show_nonprinting || show_tabs)) {
        return strdup(s);
    }

    char *buf = malloc(strlen(s) * 5 + 1); // Worst case
    if (!buf) return NULL;
    
    char *p = buf;
    for (; *s; s++) {
        unsigned char c = *s;
        if (c == '\t' && show_tabs) {
            *p++ = '^';
            *p++ = 'I';
        } else if (c >= 32 && c < 127) {
            *p++ = c;
        } else if (c == 127) {
            *p++ = '^';
            *p++ = '?';
        } else if (c < 32) {
            *p++ = '^';
            *p++ = c + 64;
        } else if (c >= 128 && c < 128+32) {
            *p++ = 'M';
            *p++ = '-';
            *p++ = '^';
            *p++ = c - 128 + 64;
        } else if (c >= 128+32 && c < 128+127) {
            *p++ = 'M';
            *p++ = '-';
            *p++ = c - 128;
        } else if (c >= 128+127) {
            *p++ = 'M';
            *p++ = '-';
            *p++ = '^';
            *p++ = '?';
        } else {
            *p++ = c;
        }
    }
    *p = '\0';
    return buf;
}

int process_with_options(FILE *input, FILE *output) {
    char *line = NULL;
    size_t len = 0;
    ssize_t read;
    int line_num = 1;
    int prev_blank = 0;

    while ((read = getline(&line, &len, input)) != -1) {
        if (read > 0 && line[read-1] == '\n') {
            line[read-1] = '\0';
            read--;
        }

        int is_blank = (read == 0);

        if (squeeze_blank && is_blank && prev_blank) {
            continue;
        }
        prev_blank = is_blank;

        if ((number && !number_nonblank) || (number_nonblank && !is_blank)) {
            fprintf(output, "%6d\t", line_num);
            line_num++;
        }

        char *processed_line = line;
        if (show_nonprinting || show_tabs) {
            processed_line = process_nonprinting(line);
            if (!processed_line) {
                free(line);
                return -1;
            }
        }

        fprintf(output, "%s", processed_line);

        if (processed_line != line) {
            free(processed_line);
        }

        if (show_ends) {
            fprintf(output, "$");
        }

        fprintf(output, "\n");
    }

    free(line);
    if (ferror(input)) {
        return -1;
    }
    return 0;
}

int process_stdin() {
    if (show_nonprinting || show_tabs || show_ends || number || number_nonblank || squeeze_blank) {
        return process_with_options(stdin, stdout);
    }
    
    char buffer[4096];
    size_t bytes;
    while ((bytes = fread(buffer, 1, sizeof(buffer), stdin)) > 0) {
        if (fwrite(buffer, 1, bytes, stdout) != bytes) {
            return -1;
        }
    }
    return ferror(stdin) ? -1 : 0;
}

int process_regular_file(const char *filename) {
    struct stat st;
    if (lstat(filename, &st) == -1) {
        return -1;
    }

    FILE *input = NULL;
    if (follow_symlinks) {
        input = fopen(filename, "r");
    } else {
        input = fopen(filename, "r");
        if (!input && errno == EACCES) {
            char resolved[PATH_MAX];
            if (realpath(filename, resolved) && strcmp(resolved, filename) != 0) {
                fprintf(stderr, "refusing to follow symlink\n");
                return -1;
            }
            input = fopen(filename, "r");
        }
    }

    if (!input) {
        return -1;
    }

    int result = 0;
    if (show_nonprinting || show_tabs || show_ends || number || number_nonblank || squeeze_blank) {
        result = process_with_options(input, stdout);
    } else {
        char buffer[4096];
        size_t bytes;
        while ((bytes = fread(buffer, 1, sizeof(buffer), input)) > 0) {
            if (fwrite(buffer, 1, bytes, stdout) != bytes) {
                result = -1;
                break;
            }
        }
        if (ferror(input)) {
            result = -1;
        }
    }

    if (fclose(input) == EOF) {
        fprintf(stderr, "warning: error closing file\n");
    }
    return result;
}

int main(int argc, char *argv[]) {
    static struct option long_options[] = {
        {"number", no_argument, NULL, 'n'},
        {"number-nonblank", no_argument, NULL, 'b'},
        {"squeeze-blank", no_argument, NULL, 's'},
        {"show-ends", no_argument, NULL, 'E'},
        {"show-nonprinting", no_argument, NULL, 'v'},
        {"show-tabs", no_argument, NULL, 'T'},
        {"show-all", no_argument, NULL, 'A'},
        {"follow-symlinks", no_argument, NULL, 'L'},
        {"version", no_argument, NULL, 0},
        {NULL, 0, NULL, 0}
    };

    int opt;
    while ((opt = getopt_long(argc, argv, "nbsEvTAtL", long_options, NULL)) != -1) {
        switch (opt) {
            case 'n': number = 1; break;
            case 'b': number_nonblank = 1; break;
            case 's': squeeze_blank = 1; break;
            case 'E': show_ends = 1; break;
            case 'v': show_nonprinting = 1; break;
            case 'T': show_tabs = 1; break;
            case 'A': show_all = 1; break;
            case 'e': show_nonprinting = 1; show_ends = 1; break;
            case 't': show_nonprinting = 1; show_tabs = 1; break;
            case 'L': follow_symlinks = 1; break;
            case 0:
                print_version(argv[0]);
                exit(0);
            default:
                usage(argv[0]);
                exit(1);
        }
    }

    handle_combined_options();

    int exit_code = 0;
    if (optind >= argc) {
        if (process_stdin() != 0) {
            fprintf(stderr, "%s: stdin: %s\n", argv[0], strerror(errno));
            exit_code = 1;
        }
    } else {
        for (int i = optind; i < argc; i++) {
            const char *filename = argv[i];
            if (strcmp(filename, "-") == 0) {
                if (process_stdin() != 0) {
                    fprintf(stderr, "%s: stdin: %s\n", argv[0], strerror(errno));
                    exit_code = 1;
                }
            } else {
                if (process_regular_file(filename) != 0) {
                    fprintf(stderr, "%s: %s: %s\n", argv[0], filename, strerror(errno));
                    exit_code = 1;
                }
            }
        }
    }

    exit(exit_code);
}