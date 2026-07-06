---
title: Generate your first world
weight: 1
---

In this tutorial we'll start a new game and generate its world. A game in T'Pty
is created first as an empty manifest, and then we fill it in — so we'll create a
game, generate a world for it, look at what we made, and see that it's
reproducible.

You'll run everything from the repository root. That's all you need.

## Create the game

A game begins as a manifest — a `game.json` file that records the game's id, its
two master seeds, and where its other files live. Create one:

```sh
go run ./cmd/tpty game create --game-id tutorial --data data/tutorial --seed1 1 --seed2 2
```

You'll see two lines:

```
created game "tutorial" (seed1=1 seed2=2)
wrote data/tutorial/game.json
```

You now have a game. It's the manifest under `data/tutorial/` — but it has no
world yet. The two seeds, `1` and `2`, are what make this *your* game: the same
seeds always produce the same world.

## Generate its world

Now fill the game in with a world:

```sh
go run ./cmd/tpty world generate --rings 3 --data data/tutorial
```

Notice there are no seeds on this command — `world generate` reads them from the
game you just created. You'll see three lines:

```
generated world for game "tutorial" (seed1=1 seed2=2)
wrote 37 provinces to data/tutorial/world.json
wrote terrain translation to data/tutorial/terrain-translation.json
```

That's it — your game now has a world. The `--rings 3` gave it three rings of
provinces around a center, which is why it has 37 of them.

## Look at what you made

Open `data/tutorial/world.json`. The top looks like this:

```json
{
  "seeds": {
    "seed1": 3277606528033075706,
    "seed2": 3513926073913791681
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
      "terrain": "Plains"
    },
```

Each entry in `provinces` is one hex of the world: `q` and `r` are its
coordinates, and `terrain` is what's there. The very first province, at `(0, 0)`,
is the center of the world — and it's always a mountain. Scroll down and you'll
find all 37, out to the edge of the third ring.

Notice the seeds here aren't the `1` and `2` you chose — the world derives its
own seeds from the game's. Your game's seeds are in `data/tutorial/game.json`.

## See that it's reproducible

Run the world command again — exactly as before:

```sh
go run ./cmd/tpty world generate --rings 3 --data data/tutorial
```

You get the same three lines, and `world.json` is byte-for-byte identical. This
is the heart of T'Pty: a game is defined by its seeds, so the same seeds rebuild
the same world every time.

(Try re-running `game create` instead and you'll see it refuse — `tpty` won't
overwrite a game that already exists. To make a *different* world, create another
game in a new directory with different seeds, for example
`--game-id tutorial-2 --data data/tutorial-2 --seed1 1 --seed2 3`, and generate
its world.)

## What you did

You created your first game, generated a world for it, inspected its provinces,
and saw that the seeds make it reproducible. You can delete `data/tutorial/` and
start over any time.

Your world is ready, but a game needs players. When you're ready to run a real
game, these guides pick up where you left off:

- [Recruit players and add them to a game]({{< relref "/docs/how-to/recruit-players.md" >}}) —
  choose the starting provinces you'll offer, then add each player
- [Generate a world]({{< relref "/docs/how-to/generate-a-world.md" >}}) — all the
  world options, once you know the basics
- [Render a world to Worldographer]({{< relref "/docs/how-to/render-world-to-worldographer.md" >}}) — see your world as a map
- [World generation reference]({{< relref "/docs/reference/world-generation.md" >}}) — the rules behind what you just made
```
