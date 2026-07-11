---
title: Grammar
weight: 1
---

A compact grammar for order lines: one line per order, giving the command word
and its parameters. See [Orders]({{< relref "/docs/reference/orders" >}}) for the
orders file format that these lines appear in, and for the command summary with
each order's time cost.

Notation:

- `<parameter>` is required.
- `[parameter]` is optional.

```
hold
move <direction>
attack [direction]
use [skill] [target] [modifier]
take <unit>
drop [unit]
join <stack>
study <skill> [days]
work [skill] [options]
buy <thing> [from] <offer> [number]
sell <thing> [to] <price> [number]
follow [entity]
explore
persuade <entity> [skill] [bribe]
swear [lord-entity]
pay <entity> <money> <moneyLeft>
declare [entity] <opinion>
recruit <numberSought> payOffered
form <armor> [speciesHired] [amount] [numOrders]
pillage <province> [severity]
tax <province> [severity]
execute <captive>
terrorize [province] [severity] [mode]
wait [days]
armor [newRating]
tell [entity] <y/n/number> [number]
garrison [state]
```

## See also

- [Orders]({{< relref "/docs/reference/orders" >}})
