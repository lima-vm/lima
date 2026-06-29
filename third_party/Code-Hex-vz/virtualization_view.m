//
//  virtualization_view.m
//
//  Created by codehex.
//

#import "virtualization_view.h"

@implementation VZApplication

- (void)run
{
    @autoreleasepool {
        [self finishLaunching];

        shouldKeepRunning = YES;
        do {
            NSEvent *event = [self
                nextEventMatchingMask:NSEventMaskAny
                            untilDate:[NSDate distantFuture]
                               inMode:NSDefaultRunLoopMode
                              dequeue:YES];
            // NSLog(@"event: %@", event);
            [self sendEvent:event];
            [self updateWindows];
        } while (shouldKeepRunning);
    }
}

- (void)terminate:(id)sender
{
    shouldKeepRunning = NO;

    // We should call this method if we want to use `applicationWillTerminate` method.
    //
    // [[NSNotificationCenter defaultCenter]
    //     postNotificationName:NSApplicationWillTerminateNotification
    //                   object:NSApp];

    // This method is used to end up the event loop.
    // If no events are coming, the event loop will always be in a waiting state.
    [self postEvent:self.currentEvent atStart:NO];
}
@end

@implementation AboutViewController

- (instancetype)init
{
    self = [super initWithNibName:nil bundle:nil];
    return self;
}

- (void)loadView
{
    self.view = [NSView new];
    NSImageView *imageView = [NSImageView imageViewWithImage:[NSApp applicationIconImage]];
    NSTextField *appLabel = [self makeLabel:[[NSProcessInfo processInfo] processName]];
    [appLabel setFont:[NSFont boldSystemFontOfSize:16]];
    NSTextField *subLabel = [self makePoweredByLabel];

    NSStackView *stackView = [NSStackView stackViewWithViews:@[
        imageView,
        appLabel,
        subLabel,
    ]];
    [stackView setOrientation:NSUserInterfaceLayoutOrientationVertical];
    [stackView setDistribution:NSStackViewDistributionFillProportionally];
    [stackView setSpacing:10];
    [stackView setAlignment:NSLayoutAttributeCenterX];
    [stackView setContentCompressionResistancePriority:NSLayoutPriorityRequired forOrientation:NSLayoutConstraintOrientationHorizontal];
    [stackView setContentCompressionResistancePriority:NSLayoutPriorityRequired forOrientation:NSLayoutConstraintOrientationVertical];

    [self.view addSubview:stackView];

    [NSLayoutConstraint activateConstraints:@[
        [imageView.widthAnchor constraintEqualToConstant:80], // image size
        [imageView.heightAnchor constraintEqualToConstant:80], // image size
        [stackView.topAnchor constraintEqualToAnchor:self.view.topAnchor
                                            constant:4],
        [stackView.bottomAnchor constraintEqualToAnchor:self.view.bottomAnchor
                                               constant:-16],
        [stackView.leadingAnchor constraintEqualToAnchor:self.view.leadingAnchor
                                                constant:32],
        [stackView.trailingAnchor constraintEqualToAnchor:self.view.trailingAnchor
                                                 constant:-32],
        [stackView.widthAnchor constraintEqualToConstant:300]
    ]];
}

- (NSTextField *)makePoweredByLabel
{
    NSMutableAttributedString *poweredByAttr = [[[NSMutableAttributedString alloc]
        initWithString:@"Powered by "
            attributes:@{
                NSForegroundColorAttributeName : [NSColor labelColor]
            }] autorelease];
    NSURL *repositoryURL = [NSURL URLWithString:@"https://github.com/Code-Hex/vz"];
    NSMutableAttributedString *repository = [self makeHyperLink:@"github.com/Code-Hex/vz" withURL:repositoryURL];
    [poweredByAttr appendAttributedString:repository];
    [poweredByAttr addAttribute:NSFontAttributeName
                          value:[NSFont systemFontOfSize:12]
                          range:NSMakeRange(0, [poweredByAttr length])];

    NSTextField *label = [self makeLabel:@""];
    [label setSelectable:YES];
    [label setAllowsEditingTextAttributes:YES];
    [label setAttributedStringValue:poweredByAttr];
    return label;
}

- (NSTextField *)makeLabel:(NSString *)label
{
    NSTextField *appLabel = [NSTextField labelWithString:label];
    [appLabel setTextColor:[NSColor labelColor]];
    [appLabel setEditable:NO];
    [appLabel setSelectable:NO];
    [appLabel setBezeled:NO];
    [appLabel setBordered:NO];
    [appLabel setBackgroundColor:[NSColor clearColor]];
    [appLabel setAlignment:NSTextAlignmentCenter];
    [appLabel setLineBreakMode:NSLineBreakByWordWrapping];
    [appLabel setUsesSingleLineMode:NO];
    [appLabel setMaximumNumberOfLines:20];
    return appLabel;
}

// https://developer.apple.com/library/archive/qa/qa1487/_index.html
- (NSMutableAttributedString *)makeHyperLink:(NSString *)inString withURL:(NSURL *)aURL
{
    NSMutableAttributedString *attrString = [[NSMutableAttributedString alloc] initWithString:inString];
    NSRange range = NSMakeRange(0, [attrString length]);

    [attrString beginEditing];
    [attrString addAttribute:NSLinkAttributeName value:[aURL absoluteString] range:range];

    // make the text appear in blue
    [attrString addAttribute:NSForegroundColorAttributeName value:[NSColor blueColor] range:range];

    // next make the text appear with an underline
    [attrString addAttribute:NSUnderlineStyleAttributeName
                       value:[NSNumber numberWithInt:NSUnderlineStyleSingle]
                       range:range];

    [attrString endEditing];
    return [attrString autorelease];
}

@end

@implementation AboutPanel

- (instancetype)init
{
    self = [super initWithContentRect:NSZeroRect styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable backing:NSBackingStoreBuffered defer:NO];

    AboutViewController *viewController = [[[AboutViewController alloc] init] autorelease];
    [self setContentViewController:viewController];
    [self setTitleVisibility:NSWindowTitleHidden];
    [self setTitlebarAppearsTransparent:YES];
    [self setBecomesKeyOnlyIfNeeded:NO];
    [self center];
    return self;
}

@end

@implementation AppDelegate {
    VZVirtualMachine *_virtualMachine;
    dispatch_queue_t _queue;
    VZVirtualMachineView *_virtualMachineView;
    NSWindow *_window;
    NSToolbar *_toolbar;
    BOOL _enableController;
    // Overlay for pause mode.
    NSVisualEffectView *_pauseOverlayView;
    // Zoom function properties.
    BOOL _isZoomEnabled;
    NSTimer *_scrollTimer;
    NSPoint _scrollDelta;
    id _mouseMovedMonitor;
}

- (instancetype)initWithVirtualMachine:(VZVirtualMachine *)virtualMachine
                                 queue:(dispatch_queue_t)queue
                           windowWidth:(CGFloat)windowWidth
                          windowHeight:(CGFloat)windowHeight
                           windowTitle:(NSString *)windowTitle
                      enableController:(BOOL)enableController
{
    self = [super init];
    _virtualMachine = virtualMachine;
    [_virtualMachine setDelegate:self];

    // Setup virtual machine view configs
    VZVirtualMachineView *view = [[[VZVirtualMachineView alloc] init] autorelease];
    view.capturesSystemKeys = YES;
    view.virtualMachine = _virtualMachine;
#ifdef INCLUDE_TARGET_OSX_14
    if (@available(macOS 14.0, *)) {
        // Configure the app to automatically respond to changes in the display size.
        view.automaticallyReconfiguresDisplay = YES;
    }
#endif
    _virtualMachineView = view;
    _queue = queue;

    // Setup some window configs
    _window = [self createMainWindowWithTitle:windowTitle width:windowWidth height:windowHeight];
    _toolbar = [self createCustomToolbar];
    _enableController = enableController;
    [_virtualMachine addObserver:self
                      forKeyPath:@"state"
                         options:NSKeyValueObservingOptionNew
                         context:nil];
    _pauseOverlayView = [self createPauseOverlayEffectView:_virtualMachineView];
    [_virtualMachineView addSubview:_pauseOverlayView];
    _isZoomEnabled = NO;
    return self;
}

- (void)dealloc
{
    if (_mouseMovedMonitor) {
        [NSEvent removeMonitor:_mouseMovedMonitor];
        _mouseMovedMonitor = nil;
    }
    [self stopScrollTimer];
    if (_virtualMachine) {
        [_virtualMachine removeObserver:self forKeyPath:@"state"];
    }
    _virtualMachineView = nil;
    _virtualMachine = nil;
    _queue = nil;
    _toolbar = nil;
    _window = nil;
    _pauseOverlayView = nil;
    [super dealloc];
}

- (BOOL)canStopVirtualMachine
{
    __block BOOL result;
    dispatch_sync(_queue, ^{
        result = _virtualMachine.canStop;
    });
    return (bool)result;
}

- (BOOL)canResumeVirtualMachine
{
    __block BOOL result;
    dispatch_sync(_queue, ^{
        result = _virtualMachine.canResume;
    });
    return (bool)result;
}

- (BOOL)canPauseVirtualMachine
{
    __block BOOL result;
    dispatch_sync(_queue, ^{
        result = _virtualMachine.canPause;
    });
    return (bool)result;
}

- (BOOL)canStartVirtualMachine
{
    __block BOOL result;
    dispatch_sync(_queue, ^{
        result = _virtualMachine.canStart;
    });
    return (bool)result;
}

- (void)observeValueForKeyPath:(NSString *)keyPath ofObject:(id)object change:(NSDictionary *)change context:(void *)context;
{
    if ([keyPath isEqualToString:@"state"]) {
        VZVirtualMachineState newState = (VZVirtualMachineState)[change[NSKeyValueChangeNewKey] integerValue];
        dispatch_async(dispatch_get_main_queue(), ^{
            [self updateToolbarItems];
            if (newState == VZVirtualMachineStatePaused) {
                [self showOverlay];
            } else {
                [self hideOverlay];
            }
            // Terminating GUI Application from Guest and Host.
            // See: https://github.com/Code-Hex/vz/issues/150
            if (newState == VZVirtualMachineStateStopped) {
                [NSApp terminate:nil];
            }
        });
    }
}

// Overlay a semi-transparent view on the VZVirtualMachineView when the virtual machine is paused.
// This provides a clear visual indication to the user that the virtual machine is in a paused state.
// The overlay is hidden when the virtual machine resumes or stops.
- (NSVisualEffectView *)createPauseOverlayEffectView:(NSView *)view
{
    NSVisualEffectView *effectView = [[[NSVisualEffectView alloc] initWithFrame:view.bounds] autorelease];
    effectView.wantsLayer = YES;
    effectView.blendingMode = NSVisualEffectBlendingModeWithinWindow;
    effectView.state = NSVisualEffectStateActive;
    effectView.alphaValue = 0.7;
    effectView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    effectView.hidden = YES;
    return effectView;
}

- (void)showOverlay
{
    if (_pauseOverlayView) {
        _pauseOverlayView.hidden = NO;
    }
}

- (void)hideOverlay
{
    if (_pauseOverlayView) {
        _pauseOverlayView.hidden = YES;
    }
}

static NSString *const ZoomToolbarIdentifier = @"Zoom";
static NSString *const PauseToolbarIdentifier = @"Pause";
static NSString *const PlayToolbarIdentifier = @"Play";
static NSString *const PowerToolbarIdentifier = @"Power";
static NSString *const SpaceToolbarIdentifier = @"Space";
static NSString *const Space2ToolbarIdentifier = @"Space2";

- (NSArray<NSToolbarItemIdentifier> *)setupToolbarItemIdentifiers
{
    NSMutableArray<NSToolbarItemIdentifier> *toolbarItems = [NSMutableArray array];
    if (_enableController) {
        if ([self canPauseVirtualMachine]) {
            [toolbarItems addObject:PauseToolbarIdentifier];
        }
        if ([self canResumeVirtualMachine]) {
            [toolbarItems addObject:SpaceToolbarIdentifier];
            [toolbarItems addObject:PlayToolbarIdentifier];
        }
        if ([self canStopVirtualMachine] || [self canStartVirtualMachine]) {
            [toolbarItems addObject:Space2ToolbarIdentifier];
            [toolbarItems addObject:PowerToolbarIdentifier];
        }
    }
    [toolbarItems addObject:NSToolbarSpaceItemIdentifier];
    [toolbarItems addObject:ZoomToolbarIdentifier];
    [toolbarItems addObject:NSToolbarFlexibleSpaceItemIdentifier];
    return [toolbarItems copy];
}

- (void)updateToolbarItems
{
    NSArray<NSToolbarItemIdentifier> *toolbarItems = [self setupToolbarItemIdentifiers];
    [self setToolBarItems:toolbarItems];
}

- (void)setToolBarItems:(NSArray<NSToolbarItemIdentifier> *)desiredItems
{
    if (_toolbar) {
        while (_toolbar.items.count > 0) {
            [_toolbar removeItemAtIndex:0];
        }

        for (NSToolbarItemIdentifier itemIdentifier in desiredItems) {
            [_toolbar insertItemWithItemIdentifier:itemIdentifier atIndex:_toolbar.items.count];
        }
    }
}

/* IMPORTANT: delegate methods are called from VM's queue */
- (void)guestDidStopVirtualMachine:(VZVirtualMachine *)virtualMachine
{
    // [NSApp performSelectorOnMainThread:@selector(terminate:) withObject:self waitUntilDone:NO];
}

- (void)virtualMachine:(VZVirtualMachine *)virtualMachine didStopWithError:(NSError *)error
{
    NSLog(@"VM %@ didStopWithError: %@", virtualMachine, error);
    [NSApp performSelectorOnMainThread:@selector(terminate:) withObject:self waitUntilDone:NO];
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification
{
    [self setupMenuBar];
    [self setupGraphicWindow];

    // These methods are required to call here. Because the menubar will be not active even if
    // application is running.
    // See: https://stackoverflow.com/questions/62739862/why-doesnt-activateignoringotherapps-enable-the-menu-bar
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    [NSApp activateIgnoringOtherApps:YES];
}

- (void)windowWillClose:(NSNotification *)notification
{
    [NSApp performSelectorOnMainThread:@selector(terminate:) withObject:self waitUntilDone:NO];
}

- (void)setupGraphicWindow
{
    // Set custom title bar
    [_window setTitlebarAppearsTransparent:YES];
    [_window setToolbar:_toolbar];
    [_window setOpaque:NO];
    [_window center];

    // Monitoring mouse movement events to control auto-scrolling behavior
    // within the virtual machine window when zoom mode is enabled.
    _mouseMovedMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskMouseMoved
                                                               handler:^NSEvent *(NSEvent *event) {
                                                                   [self handleMouseMovement:event];
                                                                   return event;
                                                               }];

    // Create scroll view for the virtual machine view
    NSScrollView *scrollView = [self createScrollViewForVirtualMachineView:_virtualMachineView];
    [_window setContentView:scrollView];

    // Configure Auto Layout constraints for VirtualMachineView to resize with the window
    [_virtualMachineView setTranslatesAutoresizingMaskIntoConstraints:NO];
    [NSLayoutConstraint activateConstraints:@[
        [_virtualMachineView.leadingAnchor constraintEqualToAnchor:_window.contentView.leadingAnchor],
        [_virtualMachineView.trailingAnchor constraintEqualToAnchor:_window.contentView.trailingAnchor],
        [_virtualMachineView.topAnchor constraintEqualToAnchor:_window.contentView.topAnchor],
        [_virtualMachineView.bottomAnchor constraintEqualToAnchor:_window.contentView.bottomAnchor]
    ]];

    NSSize sizeInPixels = [self getVirtualMachineSizeInPixels];
    if (!NSEqualSizes(sizeInPixels, NSZeroSize)) {
        // setContentAspectRatio is used to maintain the aspect ratio when the user resizes the window.
        [_window setContentAspectRatio:sizeInPixels];

        // setContentSize is used to set the initial window size based on the calculated aspect ratio.
        CGFloat windowWidth = _window.frame.size.width;
        CGFloat initialHeight = windowWidth * (sizeInPixels.height / sizeInPixels.width);
        [_window setContentSize:NSMakeSize(windowWidth, initialHeight)];
    }

    [_window setDelegate:self];
    [_window makeKeyAndOrderFront:nil];

    // This code to prevent crash when called applicationShouldTerminateAfterLast_windowClosed.
    // https://stackoverflow.com/a/13470694
    [_window setReleasedWhenClosed:NO];
}

// Adjust the window content aspect ratio to match the graphics device resolution
// configured for the virtual machine. This ensures that the display output from
// the virtual machine is rendered with the correct proportions, avoiding any
// distortion within the window.
- (NSSize)getVirtualMachineSizeInPixels
{
    __block NSSize sizeInPixels = NSZeroSize;
#ifdef INCLUDE_TARGET_OSX_14
    if (@available(macOS 14.0, *)) {
        dispatch_sync(_queue, ^{
            if (_virtualMachine.graphicsDevices.count > 0) {
                VZGraphicsDevice *graphicsDevice = _virtualMachine.graphicsDevices[0];
                if (graphicsDevice.displays.count > 0) {
                    VZGraphicsDisplay *displayConfig = graphicsDevice.displays[0];
                    sizeInPixels = displayConfig.sizeInPixels;
                }
            }
        });
    }
#endif
    return sizeInPixels;
}

- (NSWindow *)createMainWindowWithTitle:(NSString *)title
                                  width:(CGFloat)width
                                 height:(CGFloat)height
{
    NSRect rect = NSMakeRect(0, 0, width, height);
    NSWindow *window = [[[NSWindow alloc] initWithContentRect:rect
                                                    styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable
                                                      backing:NSBackingStoreBuffered
                                                        defer:NO] autorelease];
    [window setTitle:title];
    return window;
}

- (NSArray<NSToolbarItemIdentifier> *)toolbarDefaultItemIdentifiers:(NSToolbar *)toolbar
{
    return [self setupToolbarItemIdentifiers];
}

- (NSArray<NSToolbarItemIdentifier> *)toolbarAllowedItemIdentifiers:(NSToolbar *)toolbar
{
    return @[
        ZoomToolbarIdentifier,
        PlayToolbarIdentifier,
        PauseToolbarIdentifier,
        SpaceToolbarIdentifier,
        Space2ToolbarIdentifier,
        PowerToolbarIdentifier,
        NSToolbarSpaceItemIdentifier,
        NSToolbarFlexibleSpaceItemIdentifier
    ];
}

- (NSToolbarItem *)toolbar:(NSToolbar *)toolbar itemForItemIdentifier:(NSToolbarItemIdentifier)itemIdentifier willBeInsertedIntoToolbar:(BOOL)flag
{
    NSToolbarItem *item = [[[NSToolbarItem alloc] initWithItemIdentifier:itemIdentifier] autorelease];

    if ([itemIdentifier isEqualToString:PauseToolbarIdentifier]) {
        [item setImage:[NSImage imageWithSystemSymbolName:@"pause.fill" accessibilityDescription:nil]];
        [item setLabel:@"Pause"];
        [item setTarget:self];
        [item setToolTip:@"Pause"];
        [item setBordered:YES];
        [item setAction:@selector(pauseButtonClicked:)];
    } else if ([itemIdentifier isEqualToString:PowerToolbarIdentifier]) {
        [item setImage:[NSImage imageWithSystemSymbolName:@"power" accessibilityDescription:nil]];
        [item setLabel:@"Power"];
        [item setTarget:self];
        [item setToolTip:@"Power ON/OFF"];
        [item setBordered:YES];
        [item setAction:@selector(powerButtonClicked:)];
    } else if ([itemIdentifier isEqualToString:PlayToolbarIdentifier]) {
        [item setImage:[NSImage imageWithSystemSymbolName:@"play.fill" accessibilityDescription:nil]];
        [item setLabel:@"Play"];
        [item setTarget:self];
        [item setToolTip:@"Resume"];
        [item setBordered:YES];
        [item setAction:@selector(playButtonClicked:)];
    } else if ([itemIdentifier isEqualToString:ZoomToolbarIdentifier]) {
        NSButton *zoomButton = [[[NSButton alloc] initWithFrame:NSMakeRect(0, 0, 40, 40)] autorelease];
        zoomButton.bezelStyle = NSBezelStyleTexturedRounded;
        [zoomButton setImage:[NSImage imageWithSystemSymbolName:@"plus.magnifyingglass" accessibilityDescription:nil]];
        [zoomButton setTarget:self];
        [zoomButton setAction:@selector(toggleZoomMode:)];
        [zoomButton setButtonType:NSButtonTypeToggle];
        [item setView:zoomButton];
        [item setLabel:@"Zoom"];
        [item setToolTip:@"Toggle Zoom"];
    } else if ([itemIdentifier isEqualToString:SpaceToolbarIdentifier] || [itemIdentifier isEqualToString:Space2ToolbarIdentifier]) {
        NSView *spaceView = [[[NSView alloc] initWithFrame:NSMakeRect(0, 0, 2, 10)] autorelease];
        item.view = spaceView;
        item.minSize = NSMakeSize(1, 10);
        item.maxSize = NSMakeSize(1, 10);
    }

    return item;
}

- (NSToolbar *)createCustomToolbar
{
    NSToolbar *toolbar = [[[NSToolbar alloc] initWithIdentifier:@"CustomToolbar"] autorelease];
    [toolbar setDelegate:self];
    [toolbar setDisplayMode:NSToolbarDisplayModeIconOnly];
    [toolbar setShowsBaselineSeparator:NO];
    [toolbar setAllowsUserCustomization:NO];
    [toolbar setAutosavesConfiguration:NO];
    return toolbar;
}

#pragma mark - Button Actions

- (void)pauseButtonClicked:(id)sender
{
    dispatch_async(dispatch_get_main_queue(), ^{
        dispatch_sync(_queue, ^{
            [_virtualMachine pauseWithCompletionHandler:^(NSError *err) {
                if (err)
                    [self showErrorAlertWithMessage:@"Failed to pause Virtual Machine" error:err];
            }];
        });
    });
}

- (void)powerButtonClicked:(id)sender
{
    dispatch_async(dispatch_get_main_queue(), ^{
        if ([self canStartVirtualMachine]) {
            dispatch_sync(_queue, ^{
                [_virtualMachine startWithCompletionHandler:^(NSError *err) {
                    if (err)
                        [self showErrorAlertWithMessage:@"Failed to start Virtual Machine" error:err];
                }];
            });
            return;
        }
        if ([self canStopVirtualMachine]) {
            NSAlert *alert = [[[NSAlert alloc] init] autorelease];
            [alert setIcon:[NSImage imageNamed:NSImageNameCaution]];
            [alert setMessageText:@"Force Stop Warning"];
            [alert setInformativeText:@"This action will stop the VM without a clean shutdown, similar to unplugging a PC.\n\nDo you want to force stop?"];
            [alert setAlertStyle:NSAlertStyleWarning];
            [alert addButtonWithTitle:@"Stop"];
            [alert addButtonWithTitle:@"Cancel"];

            NSModalResponse response = [alert runModal];
            if (response != NSAlertFirstButtonReturn) {
                return;
            }
            dispatch_sync(_queue, ^{
                [_virtualMachine stopWithCompletionHandler:^(NSError *err) {
                    if (err)
                        [self showErrorAlertWithMessage:@"Failed to stop Virtual Machine" error:err];
                }];
            });
            return;
        }
    });
}

- (void)playButtonClicked:(id)sender
{
    dispatch_async(dispatch_get_main_queue(), ^{
        dispatch_sync(_queue, ^{
            [_virtualMachine resumeWithCompletionHandler:^(NSError *err) {
                if (err)
                    [self showErrorAlertWithMessage:@"Failed to resume Virtual Machine" error:err];
            }];
        });
    });
}

- (void)showErrorAlertWithMessage:(NSString *)message error:(NSError *)error
{
    dispatch_async(dispatch_get_main_queue(), ^{
        NSAlert *alert = [[[NSAlert alloc] init] autorelease];
        [alert setMessageText:message];
        [alert setInformativeText:[NSString stringWithFormat:@"Error: %@\nCode: %ld", [error localizedDescription], (long)[error code]]];
        [alert setAlertStyle:NSAlertStyleCritical];
        [alert addButtonWithTitle:@"OK"];
        [alert runModal];
    });
}

#pragma mark - Zoom Function

- (void)toggleZoomMode:(id)sender
{
    _isZoomEnabled = !_isZoomEnabled;

    // Reset zoom when zoom mode is disabled.
    if (!_isZoomEnabled) {
        [NSAnimationContext
            runAnimationGroup:^(NSAnimationContext *context) {
                [context setDuration:0.3];
                [[_window.contentView animator] setMagnification:1.0];
            }
            completionHandler:nil];
    }
}

- (NSScrollView *)createScrollViewForVirtualMachineView:(VZVirtualMachineView *)view
{
    NSScrollView *scrollView = [[[NSScrollView alloc] initWithFrame:_window.contentView.bounds] autorelease];
    scrollView.hasVerticalScroller = YES;
    scrollView.hasHorizontalScroller = YES;
    scrollView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    scrollView.documentView = view;

    scrollView.allowsMagnification = YES;
    scrollView.maxMagnification = 4.0;
    scrollView.minMagnification = 1.0;

    // Pinch to zoom. Register the NSMagnificationGestureRecognizer
    NSMagnificationGestureRecognizer *magnifyRecognizer = [[[NSMagnificationGestureRecognizer alloc] initWithTarget:self action:@selector(handleMagnification:)] autorelease];

    // Set `delaysMagnificationEvents` to NO to ensure that pinch-to-zoom gestures
    // are immediately propagated to the VZVirtualMachineView. the default value is YES.
    //
    // If set to YES, the magnification gesture recognizer delays event handling, preventing
    // pinch-in and pinch-out interactions within the virtual machine view.
    // See: https://developer.apple.com/documentation/appkit/nsmagnificationgesturerecognizer
    magnifyRecognizer.delaysMagnificationEvents = NO;

    [scrollView addGestureRecognizer:magnifyRecognizer];

    return scrollView;
}

// Handles pinch-to-zoom gestures for the virtual machine view.
// If zoom mode is enabled, adjusts the magnification of the content view
// based on the user's pinch gesture, allowing smooth zoom in/out.
- (void)handleMagnification:(NSMagnificationGestureRecognizer *)recognizer
{
    if (!_isZoomEnabled) {
        return;
    }

    NSScrollView *scrollView = (NSScrollView *)recognizer.view;
    CGFloat newMagnification = scrollView.magnification + recognizer.magnification;
    newMagnification = MIN(scrollView.maxMagnification, MAX(scrollView.minMagnification, newMagnification));

    NSPoint locationInView = [recognizer locationInView:scrollView];
    NSPoint centeredPoint = [scrollView.contentView convertPoint:locationInView fromView:scrollView];

    [scrollView setMagnification:newMagnification centeredAtPoint:centeredPoint];
}

// When the mouse approaches the window's edges, this handler adjusts the scroll position
// to provide a smooth panning experience without requiring manual scroll input.
- (void)handleMouseMovement:(NSEvent *)event
{
    if (!_isZoomEnabled) {
        [self stopScrollTimer];
        return;
    }

    NSScrollView *scrollView = (NSScrollView *)_window.contentView;
    if (![scrollView isKindOfClass:[NSScrollView class]]) {
        [self stopScrollTimer];
        return;
    }

    // Take the mouse position.
    NSPoint mouseLocation = [scrollView.window convertPointToScreen:event.locationInWindow];
    NSRect windowFrame = scrollView.window.frame;

    const CGFloat margin = 24.0; // Set scrolling boundary margins.
    const CGFloat baseScrollSpeed = 5.0; // Basic scrolling speed

    // Calculate scroll direction and speed from here.
    _scrollDelta = NSMakePoint(0, 0);

    // X-axis scrollmeter
    if (mouseLocation.x < NSMinX(windowFrame) + margin) {
        _scrollDelta.x = -baseScrollSpeed;
    } else if (mouseLocation.x > NSMaxX(windowFrame) - margin) {
        _scrollDelta.x = baseScrollSpeed;
    }

    CGFloat titleBarHeight = scrollView.window.frame.size.height - scrollView.window.contentView.frame.size.height;

    // Y-axis scrollmeter
    // No Y-axis scrolling when the mouse is in the title bar area.
    if (mouseLocation.y >= (NSMaxY(windowFrame) - titleBarHeight)) {
        _scrollDelta.y = 0;
    } else if (mouseLocation.y < NSMinY(windowFrame) + margin) {
        _scrollDelta.y = -baseScrollSpeed;
    } else if (mouseLocation.y > NSMaxY(windowFrame) - margin - titleBarHeight) {
        _scrollDelta.y = baseScrollSpeed;
    }

    // Start timer if scrolling is required, stop if not required.
    if (_scrollDelta.x != 0 || _scrollDelta.y != 0) {
        [self startScrollTimer];
    } else {
        [self stopScrollTimer];
    }
}

- (void)startScrollTimer
{
    if (_scrollTimer == nil) {
        _scrollTimer = [NSTimer scheduledTimerWithTimeInterval:1.0 / 60.0
                                                        target:self
                                                      selector:@selector(scrollTick:)
                                                      userInfo:nil
                                                       repeats:YES];
    }
}

- (void)stopScrollTimer
{
    [_scrollTimer invalidate];
    _scrollTimer = nil;
}

- (void)scrollTick:(NSTimer *)timer
{
    NSScrollView *scrollView = (NSScrollView *)_window.contentView;
    if (![scrollView isKindOfClass:[NSScrollView class]]) {
        [self stopScrollTimer];
        return;
    }

    NSClipView *clipView = scrollView.contentView;
    NSPoint currentOrigin = clipView.bounds.origin;

    // Calculate new scroll position
    currentOrigin.x += _scrollDelta.x;
    currentOrigin.y += _scrollDelta.y;

    // Scroll position controlled.
    currentOrigin.x = MAX(0, MIN(currentOrigin.x, clipView.documentView.frame.size.width - clipView.bounds.size.width));
    currentOrigin.y = MAX(0, MIN(currentOrigin.y, clipView.documentView.frame.size.height - clipView.bounds.size.height));

    // Update scroll position
    [clipView setBoundsOrigin:currentOrigin];
}

#pragma mark - Application Menu Bar

- (void)setupMenuBar
{
    NSMenu *menuBar = [[[NSMenu alloc] init] autorelease];
    NSMenuItem *menuBarItem = [[[NSMenuItem alloc] init] autorelease];
    [menuBar addItem:menuBarItem];
    [NSApp setMainMenu:menuBar];

    // App menu
    NSMenu *appMenu = [self setupApplicationMenu];
    [menuBarItem setSubmenu:appMenu];

    // Window menu
    NSMenu *windowMenu = [self setupWindowMenu];
    NSMenuItem *windowMenuItem = [[[NSMenuItem alloc] initWithTitle:@"Window" action:nil keyEquivalent:@""] autorelease];
    [menuBar addItem:windowMenuItem];
    [windowMenuItem setSubmenu:windowMenu];

    // Help menu
    NSMenu *helpMenu = [self setupHelpMenu];
    NSMenuItem *helpMenuItem = [[[NSMenuItem alloc] initWithTitle:@"Help" action:nil keyEquivalent:@""] autorelease];
    [menuBar addItem:helpMenuItem];
    [helpMenuItem setSubmenu:helpMenu];
}

- (NSMenu *)setupApplicationMenu
{
    NSMenu *appMenu = [[[NSMenu alloc] init] autorelease];
    NSString *applicationName = [[NSProcessInfo processInfo] processName];

    NSMenuItem *aboutMenuItem = [[[NSMenuItem alloc]
        initWithTitle:[NSString stringWithFormat:@"About %@", applicationName]
               action:@selector(openAboutWindow:)
        keyEquivalent:@""] autorelease];

    // CapturesSystemKeys toggle
    NSMenuItem *capturesSystemKeysItem = [[[NSMenuItem alloc]
        initWithTitle:@"Enable to send system hot keys to virtual machine"
               action:@selector(toggleCapturesSystemKeys:)
        keyEquivalent:@""] autorelease];
    [capturesSystemKeysItem setState:[self capturesSystemKeysState]];

    // Service menu
    NSMenuItem *servicesMenuItem = [[[NSMenuItem alloc] initWithTitle:@"Services" action:nil keyEquivalent:@""] autorelease];
    NSMenu *servicesMenu = [[[NSMenu alloc] initWithTitle:@"Services"] autorelease];
    [servicesMenuItem setSubmenu:servicesMenu];
    [NSApp setServicesMenu:servicesMenu];

    NSMenuItem *hideOthersItem = [[[NSMenuItem alloc]
        initWithTitle:@"Hide Others"
               action:@selector(hideOtherApplications:)
        keyEquivalent:@"h"] autorelease];
    [hideOthersItem setKeyEquivalentModifierMask:(NSEventModifierFlagOption | NSEventModifierFlagCommand)];

    NSArray *menuItems = @[
        aboutMenuItem,
        [NSMenuItem separatorItem],
        capturesSystemKeysItem,
        [NSMenuItem separatorItem],
        servicesMenuItem,
        [NSMenuItem separatorItem],
        [[[NSMenuItem alloc]
            initWithTitle:[@"Hide " stringByAppendingString:applicationName]
                   action:@selector(hide:)
            keyEquivalent:@"h"] autorelease],
        hideOthersItem,
        [NSMenuItem separatorItem],
        [[[NSMenuItem alloc]
            initWithTitle:[@"Quit " stringByAppendingString:applicationName]
                   action:@selector(terminate:)
            keyEquivalent:@"q"] autorelease],
    ];
    for (NSMenuItem *menuItem in menuItems) {
        [appMenu addItem:menuItem];
    }
    return appMenu;
}

- (NSMenu *)setupWindowMenu
{
    NSMenu *windowMenu = [[[NSMenu alloc] initWithTitle:@"Window"] autorelease];
    NSArray *menuItems = @[
        [[[NSMenuItem alloc] initWithTitle:@"Minimize" action:@selector(performMiniaturize:) keyEquivalent:@"m"] autorelease],
        [[[NSMenuItem alloc] initWithTitle:@"Zoom" action:@selector(performZoom:) keyEquivalent:@""] autorelease],
        [NSMenuItem separatorItem],
        [[[NSMenuItem alloc] initWithTitle:@"Bring All to Front" action:@selector(arrangeInFront:) keyEquivalent:@""] autorelease],
    ];
    for (NSMenuItem *menuItem in menuItems) {
        [windowMenu addItem:menuItem];
    }
    [NSApp setWindowsMenu:windowMenu];
    return windowMenu;
}

- (NSMenu *)setupHelpMenu
{
    NSMenu *helpMenu = [[[NSMenu alloc] initWithTitle:@"Help"] autorelease];
    NSArray *menuItems = @[
        [[[NSMenuItem alloc] initWithTitle:@"Report issue" action:@selector(reportIssue:) keyEquivalent:@""] autorelease],
    ];
    for (NSMenuItem *menuItem in menuItems) {
        [helpMenu addItem:menuItem];
    }
    [NSApp setHelpMenu:helpMenu];
    return helpMenu;
}

- (void)toggleCapturesSystemKeys:(id)sender
{
    NSMenuItem *item = (NSMenuItem *)sender;
    _virtualMachineView.capturesSystemKeys = !_virtualMachineView.capturesSystemKeys;
    [item setState:[self capturesSystemKeysState]];
}

- (NSControlStateValue)capturesSystemKeysState
{
    return _virtualMachineView.capturesSystemKeys ? NSControlStateValueOn : NSControlStateValueOff;
}

- (void)reportIssue:(id)sender
{
    NSString *url = @"https://github.com/Code-Hex/vz/issues/new";
    [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:url]];
}

- (void)openAboutWindow:(id)sender
{
    AboutPanel *aboutPanel = [[[AboutPanel alloc] init] autorelease];
    [aboutPanel makeKeyAndOrderFront:nil];
}
@end
