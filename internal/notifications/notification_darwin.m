//go:build darwin

#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

static NSString * const PuushNotificationURLKey = @"puush.notification.url";

static void openNotificationURL(NSString *urlString)
{
    if (urlString.length == 0) {
        return;
    }

    NSURL *url = [NSURL URLWithString:urlString];
    if (url != nil) {
        [[NSWorkspace sharedWorkspace] openURL:url];
    }
}

@interface PuushNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation PuushNotificationDelegate

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions options))completionHandler
{
    if (@available(macOS 10.14, *)) {
        completionHandler(UNNotificationPresentationOptionAlert |
                          UNNotificationPresentationOptionSound);
    }
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
 didReceiveNotificationResponse:(UNNotificationResponse *)response
          withCompletionHandler:(void (^)(void))completionHandler
{
    if (@available(macOS 10.14, *)) {
        openNotificationURL(response.notification.request.content.userInfo[PuushNotificationURLKey]);
    }
    completionHandler();
}

@end

static PuushNotificationDelegate *puushNotificationDelegate;

static PuushNotificationDelegate *notificationDelegate(void)
{
    if (puushNotificationDelegate == nil) {
        puushNotificationDelegate = [PuushNotificationDelegate new];
    }
    return puushNotificationDelegate;
}

void puushConfigureNotificationDelegate(void)
{
    @autoreleasepool {
        PuushNotificationDelegate *delegate = notificationDelegate();
        [UNUserNotificationCenter currentNotificationCenter].delegate = delegate;
    }
}

static NSString *notificationString(const char *value)
{
    if (value == NULL) {
        return @"";
    }
    return [NSString stringWithUTF8String:value] ?: @"";
}

bool puushPostNotification(const char *cTitle, const char *cSubtitle, const char *cMessage,
                           const char *cSoundName, const char *cActionURL)
{
    @autoreleasepool {
        if ([NSBundle mainBundle].bundleIdentifier.length == 0) {
            return false;
        }

        NSString *title = notificationString(cTitle);
        NSString *subtitle = notificationString(cSubtitle);
        NSString *message = notificationString(cMessage);
        NSString *soundName = notificationString(cSoundName);
        NSString *actionURL = notificationString(cActionURL);

        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        center.delegate = notificationDelegate();

        UNMutableNotificationContent *content = [UNMutableNotificationContent new];
        content.title = title;
        content.subtitle = subtitle;
        content.body = message;
        if (soundName.length > 0) {
            content.sound = [UNNotificationSound soundNamed:soundName];
        }
        if (actionURL.length > 0) {
            content.userInfo = @{PuushNotificationURLKey: actionURL};
        }

        NSString *identifier = [[NSUUID UUID] UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
                                                                                  content:content
                                                                                  trigger:nil];
        UNAuthorizationOptions options = UNAuthorizationOptionAlert;
        if (soundName.length > 0) {
            options |= UNAuthorizationOptionSound;
        }

        [center requestAuthorizationWithOptions:options
                              completionHandler:^(BOOL granted, NSError *error) {
            if (error != nil) {
                NSLog(@"Could not authorize puush notifications: %@", error);
                return;
            }
            if (!granted) {
                NSLog(@"puush notifications were not authorized");
                return;
            }
            [center addNotificationRequest:request
                      withCompletionHandler:^(NSError *error) {
                if (error != nil) {
                    NSLog(@"Could not deliver puush notification: %@", error);
                }
            }];
        }];
        [content release];
        return true;

    }
}
