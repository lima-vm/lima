#!/bin/sh
set -eux

# This script is only intended for the default.yaml image, which is based on Ubuntu LTS

if [ "${LIMA_CIDATA_NAME}" = "default" ] && command -v patch >/dev/null 2>&1 && grep -q color_prompt "${LIMA_CIDATA_HOME}/.bashrc"; then

	! grep -q "^# Lima PS1" "${LIMA_CIDATA_HOME}/.bashrc" || exit 0

	# Change the default shell prompt from "green" to "lime" (#BFFF00)

	patch --forward -r - "${LIMA_CIDATA_HOME}/.bashrc" <<'EOF'
@@ -37,7 +37,11 @@
 
 # set a fancy prompt (non-color, unless we know we "want" color)
 case "$TERM" in
-    xterm-color|*-256color) color_prompt=yes;;
+    xterm-color) color_prompt=yes;;
+    *-256color)  color_prompt=256;;
+esac
+case "$COLORTERM" in
+    truecolor) color_prompt=true;;
 esac
 
 # uncomment for a colored prompt, if the terminal has the capability; turned
@@ -56,8 +60,13 @@
     fi
 fi
 
-if [ "$color_prompt" = yes ]; then
-    PS1='${debian_chroot:+($debian_chroot)}\[\033[01;32m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
+# Lima PS1: set color to lime
+if [ "$color_prompt" = true ]; then
+    PS1='${debian_chroot:+($debian_chroot)}\[\033[38;2;192;255;0m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
+elif [ "$color_prompt" = 256 ]; then
+    PS1='${debian_chroot:+($debian_chroot)}\[\033[38;5;154m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
+elif [ "$color_prompt" = yes ]; then
+    PS1='${debian_chroot:+($debian_chroot)}\[\033[01;92m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
 else
     PS1='${debian_chroot:+($debian_chroot)}\u@\h:\w\$ '
 fi
fi
EOF

fi
