#ifndef OPA_SIGNAL_H
#define OPA_SIGNAL_H

#include "../std.h"

#ifdef __cplusplus
extern "C" {
#endif

#define raise(signal) opa_abort("signal")

#ifdef __cplusplus
}
#endif

#endif
