FROM alpine:3.14

COPY static /static
COPY ./neuralnexus-api .

CMD ["./neuralnexus-api"]
