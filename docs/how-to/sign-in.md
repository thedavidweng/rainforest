# Sign In, Switch Accounts, or Refresh a Session

1. Open **Settings** and choose your **Marketplace Region**.
2. Click **Sign in to Amazon**. Complete the sign-in and any MFA/challenges on
   Amazon's website in the separate window.
3. Wait for **Signed in**. Rainforest checks account read access and then saves
   the session. Subsequent upload target probes and writes are confirmed separately.

To switch accounts, repeat these steps. Each attempt starts with a fresh Amazon
session inside Rainforest. Closing the Amazon window or clicking **Cancel sign-in**
leaves your existing account unchanged. **Sign Out** removes the saved credentials.
Backups cannot execute without a verified current account and target.

If the page fails to load, reconnect and retry. A rejected session (401) needs
fresh sign-in on the selected marketplace. 403 means access is denied for this
account; retrying the same credentials will not grant access. For 429, timeout,
or a network outage, wait/reconnect and retry. No failed sign-in overwrites the
previous account or changes upload destinations.

For existing exports only, expand **Import session cookies (advanced)**, select
the marketplace, then import a Cookie header, JSON or Netscape cookie file. These
are credentials: do not share them. Import is configuration, not proof of a live
session; run **Run Doctor** afterward. Cookies are stored in the permission-
restricted local vault, not encrypted by the OS Keychain.

The app language follows your system preferences (English/Simplified Chinese).
Choosing Chinese does not switch your Amazon marketplace to China. Experimental
region warnings remain visible and never disable sign-in or destination selection.
