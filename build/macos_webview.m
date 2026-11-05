#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

@interface PometiAppDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate>
@property (strong, nonatomic) NSWindow *window;
@property (strong, nonatomic) WKWebView *webView;
@property (strong, nonatomic) NSTask *serverTask;
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
    [self setupMainMenu];
    NSString *urlString = [self startLocalServer];

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
