#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// WebKit učitava samo ikonice instaliranih aplikacija, bez izlaganja datoteka HTTP poslužitelju.
@interface PometiIconHandler : NSObject <WKURLSchemeHandler>
@end

@implementation PometiIconHandler

- (void)webView:(WKWebView *)webView startURLSchemeTask:(id<WKURLSchemeTask>)task {
    NSURLComponents *components = [NSURLComponents componentsWithURL:task.request.URL resolvingAgainstBaseURL:NO];
    NSString *path = nil;
    for (NSURLQueryItem *item in components.queryItems) {
        if ([item.name isEqualToString:@"path"]) {
            path = item.value;
            break;
        }
    }
    NSString *resolved = [[path stringByStandardizingPath] stringByResolvingSymlinksInPath];
    BOOL allowed = NO;
    for (NSString *root in @[@"/Applications", [NSHomeDirectory() stringByAppendingPathComponent:@"Applications"]]) {
        NSString *prefix = [[[root stringByStandardizingPath] stringByResolvingSymlinksInPath] stringByAppendingString:@"/"];
        if ([resolved hasPrefix:prefix]) allowed = YES;
    }
    BOOL directory = NO;
    if (![components.host isEqualToString:@"application"] || !path.isAbsolutePath || !allowed ||
        ![[resolved pathExtension] isEqualToString:@"app"] ||
        ![[NSFileManager defaultManager] fileExistsAtPath:resolved isDirectory:&directory] || !directory) {
        [task didFailWithError:[NSError errorWithDomain:NSURLErrorDomain code:NSURLErrorFileDoesNotExist userInfo:nil]];
        return;
    }

    NSBundle *bundle = [NSBundle bundleWithPath:resolved];
    NSString *infoPath = [resolved stringByAppendingPathComponent:@"Contents/Info.plist"];
    NSData *infoData = [NSData dataWithContentsOfFile:infoPath];
    id metadata = infoData ? [NSPropertyListSerialization propertyListWithData:infoData options:NSPropertyListImmutable format:NULL error:nil] : nil;
    NSDictionary *info = [metadata isKindOfClass:NSDictionary.class] ? metadata : @{};
    NSImage *icon = nil;
    for (NSString *key in @[@"CFBundleIconFile", @"CFBundleIconName"]) {
        id value = info[key];
        if (![value isKindOfClass:NSString.class] || ![value length]) continue;
        NSString *name = value;
        NSString *file = name.pathExtension.length ? name : [name stringByAppendingPathExtension:@"icns"];
        NSString *resources = [resolved stringByAppendingPathComponent:@"Contents/Resources"];
        NSString *iconPath = [[resources stringByAppendingPathComponent:file] stringByResolvingSymlinksInPath];
        if ([iconPath hasPrefix:[resources stringByAppendingString:@"/"]]) {
            icon = [[NSImage alloc] initWithContentsOfFile:iconPath];
        }
        if (!icon) icon = [bundle imageForResource:name];
        if (icon) break;
    }
    if (!icon) icon = [[NSWorkspace sharedWorkspace] iconForFile:resolved];
    NSBitmapImageRep *bitmap = [[NSBitmapImageRep alloc] initWithBitmapDataPlanes:NULL
        pixelsWide:80 pixelsHigh:80 bitsPerSample:8 samplesPerPixel:4 hasAlpha:YES
        isPlanar:NO colorSpaceName:NSCalibratedRGBColorSpace bytesPerRow:0 bitsPerPixel:0];
    NSGraphicsContext *context = [NSGraphicsContext graphicsContextWithBitmapImageRep:bitmap];
    NSRect bounds = NSMakeRect(0, 0, 80, 80);
    CGImageRef image = [icon CGImageForProposedRect:&bounds context:context hints:nil];
    if (!image) {
        [task didFailWithError:[NSError errorWithDomain:NSURLErrorDomain code:NSURLErrorCannotDecodeContentData userInfo:nil]];
        return;
    }
    CGContextDrawImage(context.CGContext, CGRectMake(0, 0, 80, 80), image);
    NSData *png = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
    if (!png) {
        [task didFailWithError:[NSError errorWithDomain:NSURLErrorDomain code:NSURLErrorCannotDecodeContentData userInfo:nil]];
        return;
    }
    NSURLResponse *response = [[NSURLResponse alloc] initWithURL:task.request.URL MIMEType:@"image/png"
        expectedContentLength:png.length textEncodingName:nil];
    [task didReceiveResponse:response];
    [task didReceiveData:png];
    [task didFinish];
}

- (void)webView:(WKWebView *)webView stopURLSchemeTask:(id<WKURLSchemeTask>)task {
    // Dohvat završava sinkrono; nema preostalih zadataka za prekid.
}

@end

// Odvojeni proces čeka izlazak ove instance pa otvara točno nadograđeni paket.
static BOOL PometiScheduleRelaunch(NSString *appPath, pid_t pid, NSError **error) {
    NSString *executable = [appPath stringByAppendingPathComponent:@"Contents/MacOS/PometiME"];
    if (![[NSFileManager defaultManager] isExecutableFileAtPath:executable]) {
        if (error) *error = [NSError errorWithDomain:NSCocoaErrorDomain code:NSFileNoSuchFileError userInfo:nil];
        return NO;
    }
    NSTask *helper = [[NSTask alloc] init];
    helper.executableURL = [NSURL fileURLWithPath:@"/bin/sh"];
    helper.arguments = @[@"-c",
        @"i=0; while [ $i -lt 100 ]; do if ! kill -0 \"$1\" 2>/dev/null; then exec /usr/bin/open -n \"$2\"; fi; sleep 0.1; i=$((i+1)); done; exit 1",
        @"pometime-relaunch", [NSString stringWithFormat:@"%d", pid], appPath];
    helper.standardInput = [NSFileHandle fileHandleWithNullDevice];
    helper.standardOutput = [NSFileHandle fileHandleWithNullDevice];
    helper.standardError = [NSFileHandle fileHandleWithNullDevice];
    return [helper launchAndReturnError:error];
}

@interface PometiAppDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate, WKScriptMessageHandlerWithReply>
@property (strong, nonatomic) NSWindow *window;
@property (strong, nonatomic) WKWebView *webView;
@property (strong, nonatomic) NSTask *serverTask;
@property (copy, nonatomic) NSString *applicationPath;
@property (strong, nonatomic) NSURL *serverURL;
@property (nonatomic) BOOL restarting;
@end

@implementation PometiAppDelegate

- (void)setupMainMenu {
    NSMenu *menubar = [[NSMenu alloc] init];
    NSMenuItem *appMenuItem = [[NSMenuItem alloc] init];
    [menubar addItem:appMenuItem];
    [NSApp setMainMenu:menubar];

    NSMenu *appMenu = [[NSMenu alloc] init];
    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Zatvori PometiME"
                                                      action:@selector(terminate:)
                                               keyEquivalent:@"q"];
    [appMenu addItem:quitItem];
    [appMenuItem setSubmenu:appMenu];
}

- (NSString *)startLocalServer {
    NSString *execDir = [[[NSBundle mainBundle] executablePath] stringByDeletingLastPathComponent];
    NSString *serverPath = [execDir stringByAppendingPathComponent:@"pometime-server"];

    self.serverTask = [[NSTask alloc] init];
    [self.serverTask setExecutableURL:[NSURL fileURLWithPath:serverPath]];
    [self.serverTask setArguments:@[@"-adresa=127.0.0.1:0"]];

    NSPipe *outPipe = [NSPipe pipe];
    [self.serverTask setStandardOutput:outPipe];

    NSError *err = nil;
    if (![self.serverTask launchAndReturnError:&err]) {
        return @"http://127.0.0.1:19845";
    }

    NSFileHandle *readHandle = [outPipe fileHandleForReading];
    NSMutableData *buf = [NSMutableData data];
    while (buf.length < 256) {
        NSData *chunk = [readHandle availableData];
        if (chunk.length == 0) break;
        [buf appendData:chunk];
        NSString *text = [[NSString alloc] initWithData:buf encoding:NSUTF8StringEncoding];
        NSRange nl = [text rangeOfString:@"\n"];
        if (nl.location != NSNotFound) {
            NSString *line = [text substringToIndex:nl.location];
            if ([line hasPrefix:@"POMETIME_URL="]) {
                return [line substringFromIndex:[@"POMETIME_URL=" length]];
            }
        }
    }
    return @"http://127.0.0.1:19845";
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    self.applicationPath = [[[NSBundle mainBundle] bundlePath] stringByResolvingSymlinksInPath];
    [self setupMainMenu];
    NSString *urlString = [self startLocalServer];
    self.serverURL = [NSURL URLWithString:urlString];

    NSRect frame = NSMakeRect(0, 0, 1120, 740);
    NSUInteger style = NSWindowStyleMaskTitled |
                       NSWindowStyleMaskClosable |
                       NSWindowStyleMaskMiniaturizable |
                       NSWindowStyleMaskResizable;

    self.window = [[NSWindow alloc] initWithContentRect:frame
                                              styleMask:style
                                                backing:NSBackingStoreBuffered
                                                  defer:NO];
    [self.window setTitle:@"PometiME — Čistač diska i aplikacija"];
    [self.window setMinSize:NSMakeSize(860, 560)];
    [self.window center];
    [self.window setDelegate:self];

    WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
    [config setURLSchemeHandler:[[PometiIconHandler alloc] init] forURLScheme:@"pometime-icon"];
    [config.userContentController addScriptMessageHandlerWithReply:self contentWorld:WKContentWorld.pageWorld name:@"pometimeRestart"];
    self.webView = [[WKWebView alloc] initWithFrame:[[self.window contentView] bounds]
                                      configuration:config];
    [self.webView setAutoresizingMask:(NSViewWidthSizable | NSViewHeightSizable)];
    [[self.window contentView] addSubview:self.webView];

    NSURL *url = [NSURL URLWithString:urlString];
    NSURLRequest *req = [NSURLRequest requestWithURL:url];
    [self.webView loadRequest:req];

    [self.window makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
}

- (void)userContentController:(WKUserContentController *)controller
     didReceiveScriptMessage:(WKScriptMessage *)message
                replyHandler:(void (^)(id, NSString *))replyHandler {
    WKSecurityOrigin *origin = message.frameInfo.securityOrigin;
    if (!message.frameInfo.isMainFrame || ![origin.protocol isEqualToString:self.serverURL.scheme] ||
        ![origin.host isEqualToString:self.serverURL.host] || origin.port != self.serverURL.port.integerValue) {
        replyHandler(nil, @"Ponovno pokretanje dopušteno je samo iz aplikacije.");
        return;
    }
    if (self.restarting) {
        replyHandler(nil, @"Ponovno pokretanje već je u tijeku.");
        return;
    }
    NSError *error = nil;
    if (!PometiScheduleRelaunch(self.applicationPath, NSProcessInfo.processInfo.processIdentifier, &error)) {
        replyHandler(nil, @"Ne mogu pripremiti ponovno pokretanje. Zatvorite PometiME i otvorite ga ponovno.");
        return;
    }
    self.restarting = YES;
    replyHandler(@YES, nil);
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 150 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
        [NSApp terminate:nil];
    });
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    return YES;
}

- (void)applicationWillTerminate:(NSNotification *)notification {
    if (self.serverTask && [self.serverTask isRunning]) {
        [self.serverTask terminate];
    }
}

@end

int main(int argc, const char * argv[]) {
    @autoreleasepool {
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyRegular];
        PometiAppDelegate *delegate = [[PometiAppDelegate alloc] init];
        [app setDelegate:delegate];
        [app run];
    }
    return 0;
}
