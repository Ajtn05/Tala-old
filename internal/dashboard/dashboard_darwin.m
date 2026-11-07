#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <AVKit/AVKit.h>
#import <AVFoundation/AVFoundation.h>
#import "dashboard.h"
#import <objc/runtime.h>
extern void goDashboardMessage(char *raw);

@interface KianWebView : WKWebView
@end
@interface KianDashboard : NSObject <WKScriptMessageHandler, WKUIDelegate, WKNavigationDelegate, NSWindowDelegate>
@property(strong) NSWindow *window;
@property(strong) KianWebView *web;
@property(strong) NSWindow *playerWindow;
@property(strong) AVPlayerView *player;
@property(copy) NSString *playingPath;
@end
static KianDashboard *host;
static void diagnostic(NSString *message){NSData *data=[NSJSONSerialization dataWithJSONObject:@{@"id":@(-1),@"action":@"diagnostic",@"data":message} options:0 error:nil];NSString *text=[[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding];goDashboardMessage((char *)text.UTF8String);}
static void sendEvent(NSString *type,id data) {
 NSData *json=[NSJSONSerialization dataWithJSONObject:@{@"type":type,@"data":data?:[NSNull null]} options:NSJSONWritingFragmentsAllowed error:nil];
 NSString *s=[[NSString alloc]initWithData:json encoding:NSUTF8StringEncoding];
 [host.web evaluateJavaScript:[NSString stringWithFormat:@"window.receive && window.receive(%@)",s] completionHandler:nil];
}
@implementation KianWebView
- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {return NSDragOperationCopy;}
- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
 NSArray *urls=[sender.draggingPasteboard readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey:@YES}];
 NSMutableArray *paths=[NSMutableArray array];for(NSURL *url in urls){[paths addObject:url.path];}
 if(paths.count){sendEvent(@"files",paths);return YES;}return NO;
}
@end
@implementation KianDashboard
- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
 if(!message.frameInfo.isMainFrame)return;
 if(![message.body isKindOfClass:[NSDictionary class]])return;
 if([message.body[@"action"] isEqual:@"preference"]){NSDictionary *data=message.body[@"data"];if([@[@"displayMode",@"fontSize",@"selectedSession"] containsObject:data[@"key"]]&&[data[@"value"] isKindOfClass:NSString.class]){[NSUserDefaults.standardUserDefaults setObject:data[@"value"] forKey:[@"KianStudio." stringByAppendingString:data[@"key"]]];}return;}
 NSData *data=[NSJSONSerialization dataWithJSONObject:message.body options:0 error:nil];
 if(data.length>4*1024*1024)return;
 NSString *s=[[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding];goDashboardMessage((char*)s.UTF8String);
}
- (void)webView:(WKWebView *)web decidePolicyForNavigationAction:(WKNavigationAction *)action decisionHandler:(void (^)(WKNavigationActionPolicy))handler {
 NSString *scheme=action.request.URL.scheme;
 handler(([scheme isEqual:@"about"]||!scheme)?WKNavigationActionPolicyAllow:WKNavigationActionPolicyCancel);
}
- (void)webView:(WKWebView *)web runJavaScriptConfirmPanelWithMessage:(NSString *)message initiatedByFrame:(WKFrameInfo *)frame completionHandler:(void (^)(BOOL))completion {
 NSAlert *alert=[NSAlert new];alert.messageText=@"Kian";alert.informativeText=message;[alert addButtonWithTitle:@"Confirm"];[alert addButtonWithTitle:@"Cancel"];
 [alert beginSheetModalForWindow:self.window completionHandler:^(NSModalResponse result){completion(result==NSAlertFirstButtonReturn);}];
}
- (void)quitStudio:(id)sender {goDashboardMessage("{\"id\":-1,\"action\":\"quit\"}");}
- (BOOL)windowShouldClose:(NSWindow *)sender {[sender orderOut:nil];return NO;}
- (void)webView:(WKWebView *)web didFailProvisionalNavigation:(WKNavigation *)nav withError:(NSError *)error {diagnostic(error.localizedDescription);}
- (void)webView:(WKWebView *)web didFailNavigation:(WKNavigation *)nav withError:(NSError *)error {diagnostic(error.localizedDescription);}
- (void)webView:(WKWebView *)web didFinishNavigation:(WKNavigation *)navigation {diagnostic(@"Dashboard loaded");goDashboardMessage("{\"id\":0,\"action\":\"snapshot\"}");}
- (void)webViewWebContentProcessDidTerminate:(WKWebView *)web {diagnostic(@"Web content process terminated");[web reload];}
@end
static BOOL studioReopen(id self,SEL cmd,NSApplication *app,BOOL visible){dashboardShow();return YES;}
void dashboardActivate(void){[[NSDistributedNotificationCenter defaultCenter] postNotificationName:@"app.getkian.studio.show" object:nil userInfo:nil deliverImmediately:YES];}
void dashboardInit(const char *html) {
 NSString *page=[NSString stringWithUTF8String:html];
 dispatch_async(dispatch_get_main_queue(),^{
 if(host)return;host=[KianDashboard new];
 class_addMethod([NSApp.delegate class],@selector(applicationShouldHandleReopen:hasVisibleWindows:),(IMP)studioReopen,"B@:@B");
 [[NSDistributedNotificationCenter defaultCenter] addObserverForName:@"app.getkian.studio.show" object:nil queue:[NSOperationQueue mainQueue] usingBlock:^(NSNotification *n){dashboardShow();}];
 WKWebViewConfiguration *configuration=[WKWebViewConfiguration new];configuration.websiteDataStore=[WKWebsiteDataStore nonPersistentDataStore];
 [configuration.userContentController addScriptMessageHandler:host name:@"kian"];
 NSMutableDictionary *preferences=[NSMutableDictionary dictionary];for(NSString *key in @[@"displayMode",@"fontSize",@"selectedSession"]){NSString *value=[NSUserDefaults.standardUserDefaults stringForKey:[@"KianStudio." stringByAppendingString:key]];if(value)preferences[key]=value;}
 NSData *prefs=[NSJSONSerialization dataWithJSONObject:preferences options:0 error:nil];NSString *prefsJSON=[[NSString alloc]initWithData:prefs encoding:NSUTF8StringEncoding];
 [configuration.userContentController addUserScript:[[WKUserScript alloc]initWithSource:[NSString stringWithFormat:@"window.kianPreferences=%@;",prefsJSON] injectionTime:WKUserScriptInjectionTimeAtDocumentStart forMainFrameOnly:YES]];
 host.web=[[KianWebView alloc]initWithFrame:NSMakeRect(0,0,1100,760) configuration:configuration];host.web.UIDelegate=host;host.web.navigationDelegate=host;
 [host.web registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
 host.window=[[NSWindow alloc]initWithContentRect:NSMakeRect(0,0,1100,760) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];
 host.window.title=@"Kian Studio";host.window.minSize=NSMakeSize(680,480);host.window.delegate=host;host.window.releasedWhenClosed=NO;
 host.window.titlebarAppearsTransparent=YES;[host.window center];[host.window setFrameAutosaveName:@"KianStudioDashboard"];
 diagnostic([NSString stringWithFormat:@"Loading %lu bytes of bundled UI",(unsigned long)page.length]);host.window.contentView=host.web;[host.web loadHTMLString:page baseURL:nil];
 // Supply standard responder-chain editing shortcuts without changing systray's delegate.
 NSMenu *main=NSApp.mainMenu;if(!main){main=[NSMenu new];NSApp.mainMenu=main;}
 NSMenuItem *applicationMenu=[NSMenuItem new];applicationMenu.submenu=[[NSMenu alloc]initWithTitle:@"Kian Studio"];
 NSMenuItem *quit=[applicationMenu.submenu addItemWithTitle:@"Quit Kian Studio" action:@selector(quitStudio:) keyEquivalent:@"q"];quit.target=host;[main addItem:applicationMenu];
 NSMenuItem *edit=[NSMenuItem new];edit.title=@"Edit";edit.submenu=[[NSMenu alloc]initWithTitle:@"Edit"];
 NSArray *titles=@[@"Undo",@"Redo",@"Cut",@"Copy",@"Paste",@"Select All"];
 NSArray *actions=@[@"undo:",@"redo:",@"cut:",@"copy:",@"paste:",@"selectAll:"];NSArray *keys=@[@"z",@"Z",@"x",@"c",@"v",@"a"];
 for(NSUInteger i=0;i<titles.count;i++){[edit.submenu addItemWithTitle:titles[i] action:NSSelectorFromString(actions[i]) keyEquivalent:keys[i]];}[main addItem:edit];
 [host.window makeKeyAndOrderFront:nil];[NSApp activateIgnoringOtherApps:YES];
 });
}
void dashboardShow(void){dispatch_async(dispatch_get_main_queue(),^{[host.window makeKeyAndOrderFront:nil];[NSApp activateIgnoringOtherApps:YES];});}
void dashboardSend(const char *json){NSString *s=[NSString stringWithUTF8String:json];dispatch_semaphore_t ack=dispatch_semaphore_create(0);dispatch_async(dispatch_get_main_queue(),^{if(host.web){[host.web evaluateJavaScript:[NSString stringWithFormat:@"window.receive && window.receive(%@)",s] completionHandler:^(id value,NSError *error){dispatch_semaphore_signal(ack);}];}else{dispatch_semaphore_signal(ack);}});dispatch_semaphore_wait(ack,dispatch_time(DISPATCH_TIME_NOW,NSEC_PER_SEC));}
void dashboardPickFiles(int model){dispatch_async(dispatch_get_main_queue(),^{NSOpenPanel *panel=[NSOpenPanel openPanel];panel.canChooseDirectories=NO;panel.allowsMultipleSelection=!model;panel.message=model?@"Choose a verified Whisper Large v3 Turbo GGML model":@"Choose audio or video files";[panel beginSheetModalForWindow:host.window completionHandler:^(NSModalResponse result){if(result==NSModalResponseOK){NSMutableArray *paths=[NSMutableArray array];for(NSURL *url in panel.URLs){[paths addObject:url.path];}sendEvent(model?@"modelFile":@"files",paths);}}];});}
void dashboardCopy(const char *text){NSString *s=[NSString stringWithUTF8String:text];dispatch_async(dispatch_get_main_queue(),^{[NSPasteboard.generalPasteboard clearContents];[NSPasteboard.generalPasteboard setString:s forType:NSPasteboardTypeString];sendEvent(@"toast",@"Transcript copied");});}
void dashboardSave(const char *name,const char *text){NSString *n=[NSString stringWithUTF8String:name],*s=[NSString stringWithUTF8String:text];dispatch_async(dispatch_get_main_queue(),^{NSSavePanel *panel=[NSSavePanel savePanel];panel.nameFieldStringValue=n;[panel beginSheetModalForWindow:host.window completionHandler:^(NSModalResponse result){if(result==NSModalResponseOK){NSError *error=nil;[s writeToURL:panel.URL atomically:YES encoding:NSUTF8StringEncoding error:&error];sendEvent(error?@"error":@"toast",error?error.localizedDescription:@"Transcript exported");}}];});}
void dashboardTop(int enabled){dispatch_async(dispatch_get_main_queue(),^{host.window.level=enabled?NSFloatingWindowLevel:NSNormalWindowLevel;});}
void dashboardPlay(const char *path,double seconds){NSString *p=[NSString stringWithUTF8String:path];dispatch_async(dispatch_get_main_queue(),^{if(!host.playerWindow){host.playerWindow=[[NSWindow alloc]initWithContentRect:NSMakeRect(0,0,640,360) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];host.playerWindow.title=@"Review media";host.playerWindow.releasedWhenClosed=NO;host.player=[[AVPlayerView alloc]initWithFrame:NSMakeRect(0,0,640,360)];host.player.controlsStyle=AVPlayerViewControlsStyleFloating;host.playerWindow.contentView=host.player;[host.playerWindow center];}
 if(![host.playingPath isEqual:p]){[host.player.player pause];host.player.player=[AVPlayer playerWithURL:[NSURL fileURLWithPath:p]];host.playingPath=p;}
 [host.player.player seekToTime:CMTimeMakeWithSeconds(seconds,600) toleranceBefore:kCMTimeZero toleranceAfter:kCMTimeZero];[host.playerWindow makeKeyAndOrderFront:nil];
 });}
