# Using torq on your phone

torq is a progressive web app: there is nothing to download from an app store. Put it on your home screen and it opens in its own window, starts instantly and looks like any other app.

## Install

| Device | Steps |
|---|---|
| **iPhone / iPad** | Open your torq address in **Safari**, tap **Share** (the square with the arrow), then **Add to Home Screen**. |
| **Android** | Open your torq address in **Chrome**, tap the menu **⋮**, then **Install app** (or **Add to Home screen**). Chrome may also offer it on its own. |
| **Desktop** | In **Chrome** or **Edge**, click the install icon at the right of the address bar, or choose **Install torq** in the browser menu. |

On phones and tablets, the dashboard shows a short hint with these steps until you install torq or dismiss it. They are also always under **Settings → About**.

> [!NOTE]
> **iPhone and iPad:** the home-screen app keeps its own storage, separate from Safari. Sign in once more after adding it.

## What you need: HTTPS

The full experience needs torq to be reached over **HTTPS** (or `localhost`):

- Chrome and Edge only offer **Install** over HTTPS.
- The **instant start** and the **"can't reach your server" screen** need a service worker, which browsers only allow over HTTPS.

Over plain HTTP, for example `http://192.168.1.20:8080` on your home network, torq works exactly as before. You can still add it to the home screen as a shortcut, but it then needs the server to start. To get HTTPS, put torq behind a reverse proxy with a certificate (Caddy, Traefik, nginx) or a tunnel, see [`deployment.md`](deployment.md).

## What is stored on your phone

Only torq itself: its page, scripts, styles and icons, so it can start without waiting for the network. **None of your data is stored on the device.** Vehicles, logbook entries, photos and exports always come from your server. Signing out leaves nothing of yours behind, which matters on a shared tablet.

Because the data lives on the server, torq needs a connection to it. Without one, it shows **"Can't reach your torq server"** with a **Try again** button.

## Updates

When you upgrade torq on your server, the app updates itself. It checks for a new version when you open it, when you return to it and every hour, and switches on your next tap to another page. If you are in the middle of entering something, it shows **"A new version of torq is available"** with **Reload now** instead, so nothing you typed is lost.

## Notifications

torq sends reminders through webhooks, for example to [ntfy](https://ntfy.sh), which has apps for iOS and Android. The home-screen app itself doesn't send notifications.
