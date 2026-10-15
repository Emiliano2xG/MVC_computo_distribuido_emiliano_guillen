FROM nginx:alpine
ARG TARGETARCH
RUN apk add --no-cache wget unzip \
 && wget -q "https://releases.hashicorp.com/consul-template/0.41.0/consul-template_0.41.0_linux_${TARGETARCH}.zip" -O /tmp/ct.zip \
 && unzip /tmp/ct.zip -d /usr/local/bin \
 && chmod +x /usr/local/bin/consul-template \
 && rm /tmp/ct.zip \
 && apk del wget unzip
COPY nginx.conf.ctmpl /etc/consul-template/nginx.conf.ctmpl
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8000
ENTRYPOINT ["/entrypoint.sh"]
