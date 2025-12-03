// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

#import "networkchange_darwin.h"

// MARK: - notify API

extern void callNotifyHandler(uintptr_t handler, int token);

uint32_t notifyRegisterDispatch(int *out_token, uintptr_t handler)
{
    dispatch_queue_t dq = dispatch_queue_create("io.lima-vm.vmnet.networkchange", DISPATCH_QUEUE_SERIAL);
    uint32_t res = notify_register_dispatch(kNotifySCNetworkChange, out_token,
        dq, ^(int token) {
            callNotifyHandler(handler, token);
        });
    dispatch_release(dq);
    return res;
}
