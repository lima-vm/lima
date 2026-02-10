// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

#pragma once

#import <notify.h>
#import <notify_keys.h>

// MARK: - Darwin notify API

uint32_t notifyRegisterDispatch(int *out_token, uintptr_t handler);
