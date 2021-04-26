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

void* operator new[](size_t size) {
    return opa_malloc(size);
}

void operator delete[](void *p) {
    opa_free(p);
}

extern "C" void __cxa_pure_virtual() {
    opa_abort("pure virtual");
}

// Instantiate the minimum set of templates part of standard libc++ ABI.
// This is required because we do not link with the libc++.

_LIBCPP_BEGIN_NAMESPACE_STD

template class _LIBCPP_CLASS_TEMPLATE_INSTANTIATION_VIS __basic_string_common<true>;
template class _LIBCPP_CLASS_TEMPLATE_INSTANTIATION_VIS basic_string<char>;
template class _LIBCPP_CLASS_TEMPLATE_INSTANTIATION_VIS basic_string<wchar_t>;

template class _LIBCPP_CLASS_TEMPLATE_INSTANTIATION_VIS __vector_base_common<true>;

template void __sort<__less<int>&, int*>(int*, int*, __less<int>&);
template bool __insertion_sort_incomplete<__less<int>&, int*>(int*, int*, __less<int>&);

_LIBCPP_END_NAMESPACE_STD
