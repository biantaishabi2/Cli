<?php

use A\B;
use C\D as E;
use Vendor\Package\{Alpha, Beta as B};

class X extends Base {
    use LoggerTrait;

    public function f() {
        $obj = new E();
        $groupAliasObj = new B();
        return $obj;
    }
}
