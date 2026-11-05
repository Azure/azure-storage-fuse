/* Copyright (c) Microsoft Corporation. Licensed under the MIT License. */
#ifndef BLOBFUSE_TELEMETRY_COUNTS_H
#define BLOBFUSE_TELEMETRY_COUNTS_H
#include <stdint.h>
#define TELEMETRY_OPERATIONS 23
#define TELEMETRY_OUTCOMES 135
void telemetry_enable(int enabled);
int telemetry_active(void);
uint64_t telemetry_count(int operation, int outcome);
int telemetry_result(int operation, int result);
#endif
