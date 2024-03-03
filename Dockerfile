FROM alpine:3.14

COPY ./build/neuralnexus-api .

CMD ["./neuralnexus-api"]
