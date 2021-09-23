FROM golang as builder

COPY . /app
RUN cd /app && make build

FROM quay.io/mocaccino/extra
ENV TEMPLATES_DIR=/usr/share/luet-package-browser
ENV CONFIG=/config.yaml
COPY templates /usr/share/luet-package-browser
COPY --from=builder /app/luet-package-browser /usr/bin/luet-package-browser
COPY config.yaml /config.yaml

ENTRYPOINT /usr/bin/luet-package-browser
