#include <algorithm>
#include <string>
#include <vector>

#include "../std.h"
#include "../malloc.h"

void* operator new(size_t size) {
    return opa_malloc(size);
}

void operator delete(void *p) {
    opa_free(p);
}

void operator delete(void *p, size_t) {
    opa_free(p);
}

void* operator new[](size_t size) {
    return opa_malloc(size);
}

void operator delete[](void *p) {
    opa_free(p);
}

void operator delete[](void *p, size_t) {
    opa_free(p);
}

extern "C" void __cxa_pure_virtual() {
    opa_abort("pure virtual");
}

// LLVM 21 verbose abort handler
_LIBCPP_BEGIN_NAMESPACE_STD
    void __libcpp_verbose_abort(const char *format, ...) noexcept {
        opa_abort(format);
    }
_LIBCPP_END_NAMESPACE_STD

// Instantiate the minimum set of templates part of standard libc++ ABI.
// This is required because we do not link with the libc++.

_LIBCPP_BEGIN_NAMESPACE_STD
template class basic_string<char>;
_LIBCPP_END_NAMESPACE_STD
