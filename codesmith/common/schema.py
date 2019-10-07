from schema import And
from box import Box


def not_empty(x):
    return bool(x)


def non_empty_string():
    return And(str, not_empty)


def box(properties, *, schema):
    return Box(schema.validate(properties), camel_killer_box=True, default_box=True, default_box_attr=None)
