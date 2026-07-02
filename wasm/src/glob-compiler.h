#ifndef OPA_GLOB_COMPILER
#define OPA_GLOB_COMPILER

#include <string>
#include <vector>

std::string glob_translate(const char *glob, size_t n, const std::vector<std::string>& delimiters, std::string *re2);

#endif
