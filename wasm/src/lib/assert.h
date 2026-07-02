#ifndef OPA_ASSERT_H
#define OPA_ASSERT_H

#include "../std.h"

#define assert(expr) ((expr) ? (void)0 : opa_abort(#expr));

#endif
