<?php

use A\B;
use C\D as E;

class X extends Base {
    use LoggerTrait;

    public function f() {
        $obj = new E();
        return $obj;
    }
}
