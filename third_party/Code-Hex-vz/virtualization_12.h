//
//  virtualization_12.h
//
//  Created by codehex.
//

#import "virtualization_helper.h"
#import "virtualization_view.h"

// FIXME(codehex): this is dirty hack to avoid clang-format error like below
// "Configuration file(s) do(es) not support C++: /github.com/Code-Hex/vz/.clang-format"
#define NSURLComponents NSURLComponents

bool vmCanStop(void *machine, void *queue);
void stopWithCompletionHandler(void *machine, void *queue, uintptr_t cgoHandle);

void *newVZGenericPlatformConfiguration();

void *newVZVirtioSoundDeviceInputStreamConfiguration();
void *newVZVirtioSoundDeviceHostInputStreamConfiguration(); // use in Go
void *newVZVirtioSoundDeviceOutputStreamConfiguration();
void *newVZVirtioSoundDeviceHostOutputStreamConfiguration(); // use in Go

void *newVZDiskImageStorageDeviceAttachmentWithCacheAndSyncMode(const char *diskPath, bool readOnly, int cacheMode, int syncMode, void **error);
void *newVZUSBScreenCoordinatePointingDeviceConfiguration();
void *newVZUSBKeyboardConfiguration();
void *newVZVirtioSoundDeviceConfiguration();
void setStreamsVZVirtioSoundDeviceConfiguration(void *audioDeviceConfiguration, void *streams);

void *newVZSharedDirectory(const char *dirPath, bool readOnly);
void *newVZSingleDirectoryShare(void *sharedDirectory);
void *newVZMultipleDirectoryShare(void *sharedDirectories);
void *newVZVirtioFileSystemDeviceConfiguration(const char *tag, void **error);
void setVZVirtioFileSystemDeviceConfigurationShare(void *config, void *share);

void setDirectorySharingDevicesVZVirtualMachineConfiguration(void *config, void *directorySharingDevices);
void setPlatformVZVirtualMachineConfiguration(void *config,
    void *platform);
void setGraphicsDevicesVZVirtualMachineConfiguration(void *config,
    void *graphicsDevices);
void setPointingDevicesVZVirtualMachineConfiguration(void *config,
    void *pointingDevices);
void setKeyboardsVZVirtualMachineConfiguration(void *config,
    void *keyboards);
void setAudioDevicesVZVirtualMachineConfiguration(void *config,
    void *audioDevices);

void startVirtualMachineWindow(void *machine, void *queue, double width, double height, const char *title, bool enableController);

/* Runtime mutation: set the directory share of the directory-sharing device at the
 * given index on a running VM. Returns NULL on success, or a static error string. */
const char *setDirectorySharingDeviceShareAtIndex(void *machine, void *queue, int index, void *share);