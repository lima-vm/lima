//
//  virtualization_12_arm64.h
//
//  Created by codehex.
//

#pragma once

#import "virtualization_helper.h"
#import <Foundation/Foundation.h>
#import <Foundation/NSNotification.h>
#import <Virtualization/Virtualization.h>

#ifdef __arm64__

@interface ProgressObserver : NSObject
- (void)observeValueForKeyPath:(NSString *)keyPath ofObject:(id)object change:(NSDictionary *)change context:(void *)context;
@end

typedef struct VZMacOSRestoreImageStruct {
    const char *url;
    const char *buildVersion;
    NSOperatingSystemVersion operatingSystemVersion;
    void *mostFeaturefulSupportedConfiguration; // (VZMacOSConfigurationRequirements *)
} VZMacOSRestoreImageStruct;

typedef struct VZMacOSConfigurationRequirementsStruct {
    uint64_t minimumSupportedCPUCount;
    uint64_t minimumSupportedMemorySize;
    void *hardwareModel; // (VZMacHardwareModel *)
} VZMacOSConfigurationRequirementsStruct;

typedef struct VZMacHardwareModelStruct {
    bool supported;
    nbyteslice dataRepresentation;
} VZMacHardwareModelStruct;

/* exported from cgo */
void macOSRestoreImageCompletionHandler(uintptr_t cgoHandle, void *restoreImage, void *errPtr);
void macOSInstallCompletionHandler(uintptr_t cgoHandle, void *errPtr);
void macOSInstallFractionCompletedHandler(uintptr_t cgoHandle, double completed);

/* Mac Configurations */
void *newVZMacPlatformConfiguration();
void *newVZMacAuxiliaryStorageWithCreating(const char *storagePath, void *hardwareModel, void **error);
void *newVZMacAuxiliaryStorage(const char *storagePath);
void *newVZMacPlatformConfiguration();
void setHardwareModelVZMacPlatformConfiguration(void *config, void *hardwareModel);
void storeHardwareModelDataVZMacPlatformConfiguration(void *config, const char *filePath);
void setMachineIdentifierVZMacPlatformConfiguration(void *config, void *machineIdentifier);
void storeMachineIdentifierDataVZMacPlatformConfiguration(void *config, const char *filePath);
void setAuxiliaryStorageVZMacPlatformConfiguration(void *config, void *auxiliaryStorage);
void *newVZMacOSBootLoader();
void *newVZMacGraphicsDeviceConfiguration();
void setDisplaysVZMacGraphicsDeviceConfiguration(void *graphicsConfiguration, void *displays);
void *newVZMacGraphicsDisplayConfiguration(NSInteger widthInPixels, NSInteger heightInPixels, NSInteger pixelsPerInch);
void *newVZMacHardwareModelWithPath(const char *hardwareModelPath);
void *newVZMacHardwareModelWithBytes(void *hardwareModelBytes, int len);
void *newVZMacMachineIdentifier();
void *newVZMacMachineIdentifierWithPath(const char *machineIdentifierPath);
void *newVZMacMachineIdentifierWithBytes(void *machineIdentifierBytes, int len);
nbyteslice getVZMacMachineIdentifierDataRepresentation(void *machineIdentifierPtr);

VZMacOSRestoreImageStruct convertVZMacOSRestoreImage2Struct(void *restoreImagePtr);
void fetchLatestSupportedMacOSRestoreImageWithCompletionHandler(uintptr_t cgoHandle);
void loadMacOSRestoreImageFile(const char *ipswPath, uintptr_t cgoHandle);

VZMacOSConfigurationRequirementsStruct convertVZMacOSConfigurationRequirements2Struct(void *requirementsPtr);
VZMacHardwareModelStruct convertVZMacHardwareModel2Struct(void *hardwareModelPtr);

void *newVZMacOSInstaller(void *virtualMachine, void *vmQueue, const char *restoreImageFilePath);
void *newProgressObserverVZMacOSInstaller();
void installByVZMacOSInstaller(void *installerPtr, void *vmQueue, void *progressObserverPtr, uintptr_t completionHandler, uintptr_t fractionCompletedHandler);
void cancelInstallVZMacOSInstaller(void *installerPtr);

#endif