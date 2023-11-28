/*
© Copyright IBM Corporation 2021, 2022

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <string.h>
#include <time.h>
#include <sys/time.h>
#include <unistd.h>

FILE *fp = NULL;
int pid;
char hostname[255];
bool debug = false;

/**
 * Determine whether debugging is enabled or not, using an environment variable.
 */
void init_debug(){
  char *debug_env = getenv("DEBUG");
  if (debug_env != NULL)
  {
    // Enable debug logging if the DEBUG environment variable is set
    if (strncmp(debug_env, "true", 4) || strncmp(debug_env, "1", 1))
    {
      debug = true;
    }
  }
}

/**
 * Internal function to initialize the log with the given file mode.
 */
int log_init_internal(char *filename, const char *mode)
{
  int result = 0;
  pid = getpid();
  hostname[254] = '\0';
  gethostname(hostname, 254);
  if (!fp)
  {
    fp = fopen(filename, "a");
    if (fp)
    {
      // Disable buffering for this file
      setbuf(fp, NULL);
    }
    else
    {
      result = 1;
    }
  }
  init_debug();
  return result;
}

int log_init_reset(char *filename)
{
  // Open the log file for writing (overwrite if it already exists)
  return log_init_internal(filename, "w");
}

int log_init(char *filename)
{
  // Open the log file file for appending
  return log_init_internal(filename, "a");
}

void log_init_file(FILE *f)
{
  fp = f;
  init_debug();
}

void log_close()
{
  if (fp)
  {
    fclose(fp);
    fp = NULL;
  }
}

void log_printf(const char *source_file, int source_line, const char *level, const char *format, ...)
{
  if (fp)
  {
    // If this is a DEBUG message, and debugging is off
    if ((strncmp(level, "DEBUG", 5) == 0) && !debug)
    {
      return;
    }
    char buf[1024] = "";
    char *cur = buf;
    char* const end = buf + sizeof buf;
    char date_buf[70];
    struct tm *utc;
    time_t t;
    struct timeval now;

    gettimeofday(&now, NULL);
    t = now.tv_sec;
    t = time(NULL);
    utc = gmtime(&t);

    cur += snprintf(cur, end-cur, "{");
    cur += snprintf(cur, end-cur, "\"loglevel\":\"%s\"", level);
    // Print ISO-8601 time and date
    if (strftime(date_buf, sizeof date_buf, "%FT%T", utc))
    {
       // Round microseconds down to milliseconds, for consistency
       cur += snprintf(cur, end-cur, ", \"ibm_datetime\":\"%s.%03ldZ\"", date_buf, now.tv_usec / (long)1000);
    }
    cur += snprintf(cur, end-cur, ", \"ibm_processId\":\"%d\"", pid);
    cur += snprintf(cur, end-cur, ", \"host\":\"%s\"", hostname);
    cur += snprintf(cur, end-cur, ", \"module\":\"%s:%d\"", source_file, source_line);
    cur += snprintf(cur, end-cur, ", \"message\":\"");

    if (strncmp(level, "DEBUG", 5) == 0)
    {
      // Add a prefix on any debug messages
      cur += snprintf(cur, end-cur, "mqhtpass: ");
    }

    // Print log message, using varargs
    va_list args;
    va_start(args, format);
    cur += vsnprintf(cur, end-cur, format, args);
    va_end(args);
    cur += snprintf(cur, end-cur, "\"}\n");

    // Important: Just do one file write, to prevent problems with multi-threading.
    // This only works if the log message is not too long for the buffer.
    fprintf(fp, "%s", buf);
  }
}

int trimmed_len(char *s, int max_len)
{
  int i;
  for (i = max_len - 1; i >= 0; i--)
  {
    if (s[i] != ' ')
      break;
  }
  return i+1;
}