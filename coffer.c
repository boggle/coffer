#include "coffer.h"
#include <stdlib.h>

void *coffer_addressof_free() {
	return &free;
}