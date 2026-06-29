package vz

// Error type returned by the Virtualization framework.
// The NSError domain is VZErrorDomain, the code is one of the ErrorCode constants.
//
//go:generate stringer -type=ErrorCode
type ErrorCode int

const (
	// ErrorInternal is an internal error such as the virtual machine unexpectedly stopping.
	ErrorInternal ErrorCode = 1 + iota

	// ErrorInvalidVirtualMachineConfiguration represents invalid machine configuration.
	ErrorInvalidVirtualMachineConfiguration

	// ErrorInvalidVirtualMachineState represents API used with a machine in the wrong state
	// (e.g. interacting with a machine before it is running).
	ErrorInvalidVirtualMachineState

	// ErrorInvalidVirtualMachineStateTransition is invalid change of state
	// (e.g. pausing a virtual machine that is not started).
	ErrorInvalidVirtualMachineStateTransition

	// ErrorInvalidDiskImage represents unrecognized disk image format or invalid disk image.
	ErrorInvalidDiskImage

	// ErrorVirtualMachineLimitExceeded represents the running virtual machine limit was exceeded.
	// Available from macOS 12.0 and above.
	ErrorVirtualMachineLimitExceeded

	// ErrorNetworkError represents network error occurred.
	// Available from macOS 13.0 and above.
	ErrorNetworkError

	// ErrorOutOfDiskSpace represents machine ran out of disk space.
	// Available from macOS 13.0 and above.
	ErrorOutOfDiskSpace

	// ErrorOperationCancelled represents the operation was cancelled.
	// Available from macOS 13.0 and above.
	ErrorOperationCancelled

	// ErrorNotSupported represents the operation is not supported.
	// Available from macOS 13.0 and above.
	ErrorNotSupported

	// ErrorSave represents the save operation failed.
	// Available from macOS 14.0 and above.
	ErrorSave

	// ErrorRestore represents the restore operation failed.
	// Available from macOS 14.0 and above.
	ErrorRestore
)

/* macOS installation errors. */
const (
	// ErrorRestoreImageCatalogLoadFailed represents the restore image catalog failed to load.
	// Available from macOS 13.0 and above.
	ErrorRestoreImageCatalogLoadFailed ErrorCode = 10001 + iota

	// ErrorInvalidRestoreImageCatalog represents the restore image catalog is invalid.
	// Available from macOS 13.0 and above.
	ErrorInvalidRestoreImageCatalog

	// ErrorNoSupportedRestoreImagesInCatalog represents the restore image catalog has no supported restore images.
	// Available from macOS 13.0 and above.
	ErrorNoSupportedRestoreImagesInCatalog

	// ErrorRestoreImageLoadFailed represents the restore image failed to load.
	// Available from macOS 13.0 and above.
	ErrorRestoreImageLoadFailed

	// ErrorInvalidRestoreImage represents the restore image is invalid.
	// Available from macOS 13.0 and above.
	ErrorInvalidRestoreImage

	// ErrorInstallationRequiresUpdate represents a software update is required to complete the installation.
	// Available from macOS 13.0 and above.
	ErrorInstallationRequiresUpdate

	// ErrorInstallationFailed is an error occurred during installation.
	// Available from macOS 13.0 and above.
	ErrorInstallationFailed
)

/* Network Block Device errors. */
const (
	// ErrorNetworkBlockDeviceNegotiationFailed represents the connection or the negotiation with the NBD server failed.
	// Available from macOS 14.0 and above.
	ErrorNetworkBlockDeviceNegotiationFailed ErrorCode = 20001 + iota

	// ErrorNetworkBlockDeviceDisconnected represents the NBD client is disconnected from the server.
	// Available from macOS 14.0 and above.
	ErrorNetworkBlockDeviceDisconnected
)

/* USB device hot-plug errors. */
const (
	// ErrorUSBControllerNotFound represents controller not found.
	// Available from macOS 15.0 and above.
	ErrorUSBControllerNotFound ErrorCode = 30001 + iota

	// ErrorDeviceAlreadyAttached represents Device is already attached.
	// Available from macOS 15.0 and above.
	ErrorDeviceAlreadyAttached

	// ErrorDeviceInitializationFailure represents device initialization failure.
	// Available from macOS 15.0 and above.
	ErrorDeviceInitializationFailure

	// ErrorDeviceNotFound represents device not found.
	// Available from macOS 15.0 and above.
	ErrorDeviceNotFound
)
