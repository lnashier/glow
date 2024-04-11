# Examples


## Draw Networks

```shell
go run . draw badnet
dot -Tsvg -O bin/badnet.gv
```

```shell
go run . draw fan1seednet
dot -Tsvg -O bin/fan1seednet.gv
```

```shell
go run . draw fannet
dot -Tsvg -O bin/fannet.gv
```

```shell
go run . draw loop1seednet
dot -Tsvg -O bin/loop1seednet.gv
```

```shell
go run . draw onewaynet
dot -Tsvg -O bin/onewaynet.gv
```

```shell
go run . draw pingpong1seednet
dot -Tsvg -O bin/pingpong1seednet.gv
```

```shell
go run . draw pingpong2seednet
dot -Tsvg -O bin/pingpong2seednet.gv
```

```shell
go run . draw subwaynet
dot -Tsvg -O bin/subwaynet.gv
```

## Run Networks

```shell
go run . up badnet
```

```shell
go run . up fan1seednet
```

```shell
go run . up fannet
```

```shell
go run . up loop1seednet
```

```shell
go run . up onewaynet
```

```shell
go run . up pingpong1seednet
```

```shell
go run . up pingpong2seednet
```

```shell
go run . up subwaynet
```
