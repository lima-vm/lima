// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package api

import "time"

// TimeSyncDriftThreshold is how far the guest clock may be from a SyncTime
// request's host_time before the guest agent sets its clock to host_time.
const TimeSyncDriftThreshold = 100 * time.Millisecond
