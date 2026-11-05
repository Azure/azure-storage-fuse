/* Copyright (c) Microsoft Corporation. Licensed under the MIT License. */
#include "telemetry_counts.h"

static uint64_t counts[TELEMETRY_OPERATIONS][TELEMETRY_OUTCOMES];
static int enabled;

void telemetry_enable(int value)
{
    __atomic_store_n(&enabled, value, __ATOMIC_RELAXED);
}

int telemetry_active(void)
{
    return __atomic_load_n(&enabled, __ATOMIC_RELAXED);
}

uint64_t telemetry_count(int operation, int outcome)
{
    if (operation < 0 || operation >= TELEMETRY_OPERATIONS ||
        outcome < 0 || outcome >= TELEMETRY_OUTCOMES)
        return 0;
    return __atomic_load_n(&counts[operation][outcome], __ATOMIC_RELAXED);
}

int telemetry_result(int operation, int result)
{
    if (operation >= 0 && operation < TELEMETRY_OPERATIONS &&
        __atomic_load_n(&enabled, __ATOMIC_RELAXED)) {
        int outcome = result >= 0 ? 0 : (result <= -134 ? 134 : -result);
        __atomic_fetch_add(&counts[operation][outcome], 1, __ATOMIC_RELAXED);
    }
    return result;
}
