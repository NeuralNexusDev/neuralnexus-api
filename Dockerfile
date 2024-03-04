FROM alpine:3.14

COPY static /static
COPY ./build/neuralnexus-api .

CMD ["./neuralnexus-api"]
