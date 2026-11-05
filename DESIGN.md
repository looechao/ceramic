# Sprinkle desktop reading

## Direction
Ivory-inspired compact reading workspace, built only with official MyGo v0.7.4. A narrow navigation rail and independent per-tab reading paths replace the earlier broad sidebar. Real content, not decoration, provides the visual focus.

## Palette
System light/dark background, surface, text, muted text and borders. Accent #37694e in light mode; #91c8a4 in dark mode. Selection uses the text color at 9% opacity with an accent glyph. No forced dark mode or custom glass. Post-type gradients remain subtle and are omitted in high contrast.

## Typography
MyGo system typography. Post body 14 pt, line height 1.55; author and column title 14 pt semibold; account 12 pt; time/metadata 11 pt. Profile title 23 pt. Selectable body and biographies retain links, emphasis and paragraphs.

## Components
Official Toolbar, SearchField, Segmented, List, Avatar, RichText, ButtonBase, MenuButton and Icon. Navigation icons retain accessible labels and shortcut tooltips. Images use 10 pt corners; link previews 7 pt; selected navigation 10 pt. Avatars are circular. Reply opens a conversation column; boost/favourite counts remain read-only, with nonzero numbers and accessible labels. Loaded images open in-app without a redundant external link beneath them.

## Layout
88 pt minimum navigation rail, expanded if native controls need more room. The hidden title bar keeps real macOS traffic lights at (12,18), with a 52 pt reserved draggable area. The account avatar or signed-out profile icon sits above Home and Search. Reading columns have 12 pt outer padding, 12 pt gaps and 12 pt rounded corners. Each tab starts with one left-aligned root column, up to 580 pt wide. Details append to the right in a horizontal scroll container that reveals the latest column. When at least 720 pt is available, two columns fit side by side; wider windows can show more. Every column has a 48 pt toolbar and independent virtual list. Posts use 14 pt vertical / 16 pt horizontal padding, 42 pt avatars, a 12 pt avatar/body gap and 6 pt content spacing. Footer actions align to the text edge with 28 pt hit height.

## Depth
White reading panels (#ffffff) float on a shared light gray workspace (#eeeef0), with a quiet 2 pt offset / 8 pt blur / 6% black shadow. Dark mode uses #232325 panels on #171719. Each panel clips its contents to rounded corners. Official MyGo VibrancySidebar (macOS NSVisualEffectView sidebar material) behind a transparent rail. Reading columns paint an opaque system background. When vibrancy is unavailable the rail paints the system surface. Controls and link previews use the system surface. The shadow belongs to the whole reading column, never individual posts. Media opens inside its originating column with a clear back action.

## Interaction rules
Home, Search and Profile each retain their own column path and horizontal offset. Authors and conversations append within the originating tab. Closing a detail removes only that column and focuses its source column, or its left neighbor when the source is gone. New searches reset only the Search path. Lists retain reading anchors; background responses belong to their originating feed, and logout invalidates them. No automatic profile prefetch or idle animation. Held reading keys request animation frames only while scrolling.

## Window sizes
Below 720 pt of content width, show only the focused column of the active tab. Default window 1240x820; minimum 420x560. Check both sides of the breakpoint, minimum height and dark/light appearances. The same state objects survive resizing.

## Future changes
Keep reading columns independently scrollable and all controls keyboard accessible. Use the existing semantic theme roles and compact type scale. Extend content types through the shared post renderer. Do not add framework patches, a local MyGo replacement, simulated AppKit controls, or write actions under a read-only authorization.

Each tab remembers a focused column. Pointer presses inside it select it without consuming child actions. Focus uses a 1 pt accent border at 35% opacity and 4 pt offset / 12 pt blur / 14% black shadow, with no layout movement. Cmd-W closes only a focused detail; on the root it closes the window.

Post types use subtle fading tints: boosts #9263c7, replies #4f89b7, warnings/sensitive #c48a38, 6.5% opacity in light and 10% in dark; ordinary posts remain untinted. High contrast omits decorative gradients. Folded warnings and sensitive media render abstract shapes with Gaussian sigma 5, not actual text or images; reveal is explicit and reversible. Server hide filters exclude posts, including nested reblogs.

Only the sidebar uses system vibrancy. Reading columns use the framework's continuous rounded corners with opaque light headers (#f7f7f9), white bodies and a gray workspace (#eeeef0). Dark mode uses #2b2b2e headers, #232325 bodies and #171719 workspace. No custom corner masks or transparent header cutouts.
