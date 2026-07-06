---
title: Generate your first world
weight: 1
---

In this tutorial you'll create your first game. In T'Pty a game begins with a
world, and **generating a world is the only way to create a new game** — so we'll
generate one, look at what we made, and see that it's reproducible.

You'll run everything from the repository root. That's all you need.

## Create a game

Run this command:

```sh
go run ./cmd/tpty world generate --rings 3 --data data/tutorial --seed1 1 --seed2 2
```

You'll see three lines:

```
seeds: seed1=1 seed2=2
wrote 37 provinces to data/tutorial/world.json
wrote terrain translation to data/tutorial/terrain-translation.json
```

That's it — you've created a game. It lives in the two files under
`data/tutorial/`. The `--rings 3` gave your world three rings of provinces around
a center, which is why it has 37 of them. The two seeds, `1` and `2`, are what
make this *your* game: the same seeds always produce the same world.

## Look at what you made

Open `data/tutorial/world.json`. The top looks like this:

```json
{
  "seeds": {
    "seed1": 1,
    "seed2": 2
  },
  "rings": 3,
  "provinces": [
    {
      "q": 0,
      "r": 0,
      "terrain": "Mountain"
    },
    {
      "q": 0,
      "r": -1,
      "terrain": "Forests"
    },
```

Each entry in `provinces` is one hex of the world: `q` and `r` are its
coordinates, and `terrain` is what's there. The very first province, at `(0, 0)`,
is the center of the world — and it's always a mountain. Scroll down and you'll
find all 37, out to the edge of the third ring.

## See that it's reproducible

Run the exact same command again:

```sh
go run ./cmd/tpty world generate --rings 3 --data data/tutorial --seed1 1 --seed2 2
```

You get the same three lines, and `world.json` is byte-for-byte identical. This
is the heart of T'Pty: a game is defined by its seeds, so the same seeds rebuild
the same world every time. Change `--seed1` or `--seed2` and you'll get a
completely different world — try `--seed1 1 --seed2 3` and look again.

## What you did

You created your first game by generating a world, inspected its provinces, and
saw that the seeds make it reproducible. You can delete `data/tutorial/` and
start over any time.

From here:

- [Generate a world]({{< ref "/docs/how-to/generate-a-world.md" >}}) — all the
  options, once you know the basics
- [Render a world to Worldographer]({{< ref "/docs/how-to/render-world-to-worldographer.md" >}}) — see your world as a map
- [World generation reference]({{< ref "/docs/reference/world-generation.md" >}}) — the rules behind what you just made
