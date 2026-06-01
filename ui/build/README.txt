If macOS says "tickr is damaged and can't be opened":

That's Apple's Gatekeeper. The app isn't signed with a paid Apple
Developer ID — it's safe, just unsigned. Fix it in one terminal command
AFTER you drag tickr to Applications:

    xattr -dr com.apple.quarantine /Applications/tickr.app

Then open the app normally. You only need to do this once.
