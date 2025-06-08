FROM scratch
COPY grlx-farmer* /farmer
ENTRYPOINT ["/farmer"] 