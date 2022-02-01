#!/bin/sh
set -eux

if [ "${LIMA_CIDATA_NAME}" = "default" ]; then

	! grep -q "^# Lima PS1" "/home/${LIMA_CIDATA_USER}.linux/.bashrc" || exit 0

	# Change the default shell prompt from "green" to "lime" (#BFFF00)

	patch --forward -r - "/home/${LIMA_CIDATA_USER}.linux/.bashrc" <<'EOF'
@@ -37,8 +37,18 @@

 # set a fancy prompt (non-color, unless we know we "want" color)
 case "$TERM" in
-    xterm-color|*-256color) color_prompt=yes;;
+    xterm-color) color_prompt=yes;;
+    *-256color)  color_prompt=256;;
 esac
+case "$COLORTERM" in
+    truecolor) color_prompt=true;;
+esac
+
+# only use the lima color on terminal background with dark theme
+case "$TERMTHEME" in
+    Dark)  lima_color=true;;
+    Light) lima_color=false;;
+ esac

 # uncomment for a colored prompt, if the terminal has the capability; turned
 # off by default to not distract the user: the focus in a terminal window
@@ -56,12 +66,18 @@
     fi
 fi

-if [ "$color_prompt" = yes ]; then
+if [ "$lima_color" = true -a "$color_prompt" = true ]; then
+    PS1='${debian_chroot:+($debian_chroot)}\[\033[38;2;192;255;0m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
+elif [ "$lima_color" = true -a "$color_prompt" = 256 ]; then
+    PS1='${debian_chroot:+($debian_chroot)}\[\033[38;5;154m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
+elif [ "$lima_color" = true -a "$color_prompt" = yes ]; then
+    PS1='${debian_chroot:+($debian_chroot)}\[\033[01;92m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
+elif [ -n "$color_prompt" ]; then
     PS1='${debian_chroot:+($debian_chroot)}\[\033[01;32m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
 else
     PS1='${debian_chroot:+($debian_chroot)}\u@\h:\w\$ '
 fi
-unset color_prompt force_color_prompt
+unset color_prompt lima_color force_color_prompt

 # If this is an xterm set the title to user@host:dir
 case "$TERM" in
EOF

fi
