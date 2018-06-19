FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
ENV DBPassword="TPB4w4TU3CkCRTQNH3MuLvKD"
ENV DamaUser="dama"
ENV DamaPassword="9e9692478ca848a19feb8e24e5506ec89"
ADD config.yml /
ADD dama-proxy /
ADD dama.pem /
ADD dama.key /
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8443
CMD ["/dama-proxy"]