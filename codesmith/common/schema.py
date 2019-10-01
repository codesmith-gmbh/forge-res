from schema import And, Schema
from box import Box


def not_empty(x):
    return bool(x)


def non_empty_string(*, field_name="Unknown"):
    return And(str, not_empty, 'field %s may not be empty'.format(field_name))


def box(properties, *, schema):
    return Box(schema.validate(properties), camel_killer_box=True)
