FROM busybox
COPY grlx-sprout* /usr/bin/grlx-sprout
ENTRYPOINT ["/usr/bin/grlx-sprout"] 