from lark import Lark, v_args, Transformer


@v_args(inline=True)  # Affects the signatures of the methods
class CalcTransformer(Transformer):
    base_sequence_val = 1

    def number(self, n):
        return int(n)

    def sum(self, e, t):
        return e + t

    def sub(self, e, t):
        return e - t

    def mul(self, e, t):
        return e * t

    def div(self, e, t):
        return e // t

    def var(self, _):
        return self.base_sequence_val


def parser(transformer):
    return Lark(
        '''
        ?start: expr
    
        ?expr: term
             | expr "+" term -> sum
             | expr "-" term -> sub
    
        ?term: factor
             | term "*" factor -> mul
             | term "/" factor -> div
    
        ?factor: NUMBER -> number
               | CNAME -> var
               | "(" expr ")" 
    
        %import common.CNAME
        %import common.NUMBER
        %import common.WS_INLINE
    
        %ignore WS_INLINE
        ''',
        parser='lalr',
        transformer=transformer
    )


def calculator(base_sequence_val):
    t = CalcTransformer()
    t.base_sequence_val = base_sequence_val
    return parser(t)
