#include <node.h>
#include <string>

#ifdef _WIN32

#include <windows.h>
#include <stdio.h>
#include <tchar.h>
#include <strsafe.h>

#else // !_WIN32

#include <cstdlib>
#include <cerrno>
#include <unistd.h>
#include <sys/wait.h>

#endif // !_WIN32

using namespace v8;


#ifdef _WIN32

int exec(const char* command) {
    STARTUPINFO si;
    PROCESS_INFORMATION pi;
    DWORD exitCode;

    ZeroMemory(&si, sizeof(si));
    si.cb = sizeof(si);
    ZeroMemory(&pi, sizeof(pi));

    // Start the child process.
    if(!CreateProcess(NULL,     // No module name (use command line)
                (LPSTR) command,// Command line
                NULL,           // Process handle not inheritable
                NULL,           // Thread handle not inheritable
                FALSE,          // Set handle inheritance to FALSE
                0,              // No creation flags
                NULL,           // Use parent's environment block
                NULL,           // Use parent's starting directory
                &si,            // Pointer to STARTUPINFO structure
                &pi)            // Pointer to PROCESS_INFORMATION structure
      )
    {
        printf("CreateProcess failed (%d).\n", GetLastError());
        return 1;
    }

    // Wait until child process exits.
    WaitForSingleObject(pi.hProcess, INFINITE);
    GetExitCodeProcess(pi.hProcess, &exitCode);

    // Close process and thread handles.
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);

    return exitCode;
}

#else // *nix

void child(const char* command) {
    char cmd[1024];
    snprintf(cmd, sizeof cmd, "%s", command);

    close(0);
    dup(1);
    close(1);
    dup(2);
    dup2(1, 2);

    execlp("sh", "sh", "-c", command, (char*)0);

    // if we get here, execlp failed
    perror("execlp");
    exit(1);
}

int parent(int pid) {
    int got_pid, status;

    while ((got_pid = wait(&status))) {   /* go to sleep until something happens */
        /* wait woke up */
        if (got_pid == pid)
            break;  /* this is what we were looking for */
        if ((got_pid == -1) && (errno != EINTR)) {
            /* an error other than an interrupted system call */
            perror("waitpid");
            return errno;
        }
    }

    if (WIFEXITED(status))  /* process exited normally */
        //printf("child process exited with value %d\n", WEXITSTATUS(status));
        return WEXITSTATUS(status);
    else if (WIFSIGNALED(status))   /* child exited on a signal */
        // printf("child process exited due to signal %d\n", WTERMSIG(status));
        return WTERMSIG(status);
    else if (WIFSTOPPED(status))    /* child was stopped */
        // printf("child process was stopped by signal %d\n", WIFSTOPPED(status));
        return WIFSTOPPED(status);

    return 1;
}

int exec(const char* command) {
    int pid;    /* process ID */

    switch (pid = fork()) {
        case 0:     /* a fork returns 0 to the child */
            child(command);
            break;

        default:    /* a fork returns a pid to the parent */
            return parent(pid);
            break;

        case -1:    /* something went wrong */
            perror("fork");
            exit(1);
    }
    exit(0);
}


#endif

std::string FlattenString(v8::Handle<v8::String> v8str) {
    v8::String::Utf8Value utf8str(v8str);
    return std::string(*utf8str, utf8str.length());
}

/**
 * Executes a command.
 */
Handle<Value> Exec(const Arguments& args) {
    HandleScope scope;

    if (args.Length() < 1) {
        return ThrowException(
            Exception::TypeError(String::New("First argument must be a string"))
        );
    }

    Local<String> str = args[0]->ToString();
    std::string cmd = FlattenString(str);
    int result = exec(cmd.c_str());

    return scope.Close(Integer::New(result));
}

void RegisterModule(Handle<Object> target) {
    target->Set(String::NewSymbol("exec"),
            FunctionTemplate::New(Exec)->GetFunction());
}

NODE_MODULE(shell, RegisterModule);
